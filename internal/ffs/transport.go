// Package ffs opens FunctionFS endpoints as an io.ReadWriter for the AA session.
package ffs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// usbReadBuf sizes each ep2 read() = one queued USB request. Must be a
// multiple of 512 (FunctionFS returns EOVERFLOW if a packet exceeds the
// queued request). mtu3 rejects anything longer than GPD_BUF_SIZE (65532)
// with EOPNOTSUPP - the gen2cp warning still prints the larger 1048572
// limit, but the queue check fails on the 65532 clause first. 256KiB reads
// never block for the version request on MT6858. 127*512 stays under it.
const usbReadBuf = 127 * 512

// FunctionFS ep0 event types (uapi linux/usb/functionfs.h, enum order).
const (
	ffsEventBind uint8 = iota
	ffsEventUnbind
	ffsEventEnable
	ffsEventDisable
	ffsEventSetup
	ffsEventSuspend
	ffsEventResume
)

var ffsEventNames = map[uint8]string{
	ffsEventBind:    "BIND",
	ffsEventUnbind:  "UNBIND",
	ffsEventEnable:  "ENABLE",
	ffsEventDisable: "DISABLE",
	ffsEventSetup:   "SETUP",
	ffsEventSuspend: "SUSPEND",
	ffsEventResume:  "RESUME",
}

// writeBusyError is returned when ep1 would block and nothing has been
// written for this call. USBWriteBusy lets the session notice it without
// importing this package.
type writeBusyError struct{}

func (writeBusyError) Error() string { return "ffs: write would block" }
func (writeBusyError) USBWriteBusy() {}

// ErrWriteBusy means ep1 would block (host not reading IN yet) and nothing
// has been written for this call. The session writer must service ctrl/pongs
// and retry - a blocking Write held the mutex for seconds while ping_rx
// continued, so DHU saw ping timeouts and tore the link down.
var ErrWriteBusy error = writeBusyError{}

// ErrEndpointReset is returned once ep0 reported DISABLE/SUSPEND/UNBIND.
// After that, endpoint FIFOs are reset; any further write would put a
// truncated/desynced TLS record on the bus (DHU BAD_DECRYPT). AACS treats
// SUSPEND the same way (session-fatal).
var ErrEndpointReset = errors.New("ffs: endpoint reset (ep0 DISABLE/SUSPEND)")

// Endpoints is the accessory FunctionFS bulk pair + ep0 event pump.
// AACS mapping: ep1 = device→host (MD writes), ep2 = host→device (MD reads).
type Endpoints struct {
	ep0 *os.File
	epW *os.File // ep1 write to HU
	epR *os.File // ep2 read from HU

	// Full-duplex: do not share one mutex across read and write.
	wMu sync.Mutex
	rMu sync.Mutex

	rbuf []byte
	roff int

	// aio: linux AIO context for ep1 (nil ⇒ sync write fallback).
	aio *aioContext

	// active flips true once the HU has actually sent us bytes. Bus events
	// before that (enumeration churn, authorized toggles, stale queued
	// DISABLEs from before we opened ep0) are normal and must NOT kill the
	// pending session - that left DHU stuck on "waiting for phone".
	active atomic.Bool
	dead   atomic.Bool
	fatal  chan error
}

// WriteWithYield is Write plus a callback invoked while an AIO write awaits
// host URB completion (session drains ctrl/pongs here). Sync fallback ignores yield.
func (e *Endpoints) WriteWithYield(p []byte, yield func()) (int, error) {
	if e.dead.Load() {
		return 0, ErrEndpointReset
	}
	if len(p) == 0 {
		return 0, nil
	}
	seq := writeSeq.Add(1)
	ch, flags, plen := peekAA(p)
	start := time.Now()
	diagLog("%d\twrite_begin\t%d\t%d\t\t\t%d\t%#x\t%d", seq, len(p), len(p)%512, ch, flags, plen)

	var (
		written int
		err     error
	)
	if e.aio != nil {
		written, err = e.writeAIO(p, yield)
	} else {
		e.wMu.Lock()
		written, err = e.writeSyncLocked(p, seq, ch, flags, plen)
		e.wMu.Unlock()
	}
	if err != nil {
		if !errors.Is(err, ErrWriteBusy) && !errors.Is(err, ErrEndpointReset) {
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			diagLog("%d\twrite_err\t%d\t%d\t%.3f\t%s\t%d\t%#x\t%d", seq, written, len(p)%512, ms, err.Error(), ch, flags, plen)
		}
		return written, err
	}
	ms := float64(time.Since(start).Microseconds()) / 1000.0
	diagLog("%d\twrite_ok\t%d\t%d\t%.3f\t\t%d\t%#x\t%d", seq, written, len(p)%512, ms, ch, flags, plen)
	return written, nil
}

// Fatal delivers the first session-fatal ep0 event (DISABLE/SUSPEND/UNBIND).
// The owner should close the endpoints and reopen for the next connection.
func (e *Endpoints) Fatal() <-chan error { return e.fatal }

// Active reports whether the HU ever sent bytes on this endpoint pair.
func (e *Endpoints) Active() bool { return e.active.Load() }

func (e *Endpoints) fail(err error) {
	if e.dead.CompareAndSwap(false, true) {
		select {
		case e.fatal <- err:
		default:
		}
	}
}

// Shutdown marks the endpoints dead and closes bulk EPs so blocked Read/Write return.
func (e *Endpoints) Shutdown(err error) {
	if err == nil {
		err = ErrEndpointReset
	}
	e.fail(err)
	_ = e.epW.Close()
	_ = e.epR.Close()
}

// OpenAccessory opens FFS bulk endpoints. ep0 must already be configured
// (SetupAccessory) and kept open by the caller or a holder process; this
// opens an additional ep0 reference for event pumping.
func OpenAccessory(ffsDir string) (*Endpoints, error) {
	ep0, err := os.OpenFile(filepath.Join(ffsDir, "ep0"), os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open ep0: %w", err)
	}
	epW, err := os.OpenFile(filepath.Join(ffsDir, "ep1"), os.O_RDWR, 0)
	if err != nil {
		_ = ep0.Close()
		return nil, fmt.Errorf("open ep1: %w", err)
	}
	// Non-blocking IN writes so a stalled host cannot pin the session writer
	// (and starve pongs) inside one Write syscall.
	if err := unix.SetNonblock(int(epW.Fd()), true); err != nil {
		_ = ep0.Close()
		_ = epW.Close()
		return nil, fmt.Errorf("ep1 nonblock: %w", err)
	}
	epR, err := os.OpenFile(filepath.Join(ffsDir, "ep2"), os.O_RDWR, 0)
	if err != nil {
		_ = ep0.Close()
		_ = epW.Close()
		return nil, fmt.Errorf("open ep2: %w", err)
	}
	e := &Endpoints{
		ep0:   ep0,
		epW:   epW,
		epR:   epR,
		rbuf:  make([]byte, 0, usbReadBuf),
		fatal: make(chan error, 1),
	}
	e.initAIO()
	go e.pumpEp0()
	return e, nil
}

func (e *Endpoints) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if e.dead.Load() {
		return 0, ErrEndpointReset
	}
	e.rMu.Lock()
	defer e.rMu.Unlock()
	for e.roff >= len(e.rbuf) {
		buf := make([]byte, usbReadBuf)
		start := time.Now()
		var n int
		var err error
		WithoutAsyncPreempt(func() { n, err = e.epR.Read(buf) })
		if n > 0 {
			first := !e.active.Swap(true)
			e.rbuf = buf[:n]
			e.roff = 0
			seq := readSeq.Add(1)
			ch, flags, plen := peekAA(buf[:n])
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			diagLog("%d\tread_ok\t%d\t%d\t%.3f\t\t%d\t%#x\t%d", seq, n, n%512, ms, ch, flags, plen)
			if first {
				head := n
				if head > 16 {
					head = 16
				}
				log.Printf("ffs ep2 first read %dB wait=%v peek ch=%d flags=%#x plen=%d head=%x",
					n, time.Since(start).Round(time.Millisecond), ch, flags, plen, buf[:head])
			}
			break
		}
		if err != nil {
			seq := readSeq.Add(1)
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			diagLog("%d\tread_err\t0\t0\t%.3f\t%s\t\t\t", seq, ms, err.Error())
			log.Printf("ffs ep2 read: %v after %v active=%v", err, time.Since(start).Round(time.Millisecond), e.active.Load())
			return 0, err
		}
	}
	n := copy(p, e.rbuf[e.roff:])
	e.roff += n
	return n, nil
}

func (e *Endpoints) Write(p []byte) (int, error) {
	return e.WriteWithYield(p, nil)
}

// writeSync is the O_NONBLOCK + retry path used when AIO is unavailable.
func (e *Endpoints) writeSync(p []byte) (int, error) {
	e.wMu.Lock()
	defer e.wMu.Unlock()
	seq := writeSeq.Add(1)
	ch, flags, plen := peekAA(p)
	return e.writeSyncLocked(p, seq, ch, flags, plen)
}

func (e *Endpoints) writeSyncLocked(p []byte, seq uint64, ch, flags uint8, plen int) (int, error) {
	// Safety net: if a partial record cannot finish, close endpoints so the
	// session reconnects instead of hanging the writer forever.
	const stallLimit = 10 * time.Second
	stall := time.AfterFunc(stallLimit, func() {
		if e.dead.Load() {
			return
		}
		log.Printf("ffs write stall >%v (seq=%d n=%d) - closing endpoints", stallLimit, seq, len(p))
		e.fail(fmt.Errorf("%w (write stall)", ErrEndpointReset))
		_ = e.epW.Close()
		_ = e.epR.Close()
	})
	defer stall.Stop()

	// AACS dataPump retries short writes until the full AA frame is out.
	// With O_NONBLOCK: if nothing is written yet, return ErrWriteBusy so the
	// session writer can drain pongs and retry. Once any byte of this buffer
	// is on the wire we must finish it (no channel interleaving mid-record).
	// NOTE: sync f_fs still blocks until host completion after queue - see
	// aio_linux.go. ErrWriteBusy only helps on mutex_trylock / not-enabled.
	written := 0
	for written < len(p) {
		if e.dead.Load() {
			return written, ErrEndpointReset
		}
		var n int
		var err error
		WithoutAsyncPreempt(func() { n, err = e.epW.Write(p[written:]) })
		if n > 0 {
			written += n
		}
		if err != nil {
			if isAgain(err) {
				if written == 0 {
					return 0, ErrWriteBusy
				}
				time.Sleep(2 * time.Millisecond)
				continue
			}
			return written, fmt.Errorf("ffs write %d/%d: %w", written, len(p), err)
		}
		if n == 0 {
			return written, fmt.Errorf("ffs write %d/%d: zero-length write", written, len(p))
		}
		if written < len(p) {
			diagLog("%d\twrite_short_retry\t%d\t%d\t\t\t%d\t%#x\t%d", seq, written, len(p)%512, ch, flags, plen)
		}
	}
	// No ZLP: AACS never writes a zero-length packet after bulk IN.
	return written, nil
}

func isAgain(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)
}

func (e *Endpoints) Close() error {
	e.closeAIO()
	var err error
	for _, f := range []*os.File{e.epR, e.epW, e.ep0} {
		if f != nil {
			if e2 := f.Close(); e2 != nil && err == nil {
				err = e2
			}
		}
	}
	return err
}

// pumpEp0 reads FunctionFS events (12-byte records: 8-byte setup union,
// 1-byte type, 3 pad). We must be the ONLY ep0 event reader - with two
// readers (old gadgetctl drained too) events raced and were silently lost.
func (e *Endpoints) pumpEp0() {
	const evSize = 12
	buf := make([]byte, evSize*16)
	var lastEnable time.Time
	for {
		var n int
		var err error
		WithoutAsyncPreempt(func() { n, err = e.ep0.Read(buf) })
		if err != nil {
			log.Printf("ffs ep0 read: %v (pump exit, hu_bytes=%v)", err, e.active.Load())
			// Unmount/unbind wakes ep0 and leaves the bulk read blocked.
			// Without fail(), the handshake ticker runs until the process
			// is killed. The car unplug on 2026-10-20 left that MD up for hours.
			e.fail(fmt.Errorf("%w (ep0 closed)", ErrEndpointReset))
			return
		}
		for off := 0; off+evSize <= n; off += evSize {
			typ := buf[off+8]
			name := ffsEventNames[typ]
			if name == "" {
				name = fmt.Sprintf("type=%d", typ)
			}
			sinceEn := "never-enabled"
			if !lastEnable.IsZero() {
				sinceEn = "+" + time.Since(lastEnable).Round(time.Millisecond).String()
			}
			log.Printf("ffs ep0 event: %s (enable %s hu_bytes=%v)", name, sinceEn, e.active.Load())
			switch typ {
			case ffsEventEnable:
				lastEnable = time.Now()
			case ffsEventDisable, ffsEventSuspend, ffsEventUnbind:
				if e.active.Load() {
					log.Printf("ffs ep0 %s after HU traffic (enable %s) - failing session", name, sinceEn)
					e.fail(fmt.Errorf("%w (%s)", ErrEndpointReset, name))
				} else {
					log.Printf("ffs ep0 %s before HU traffic (enable %s) - ignoring (enumeration churn)", name, sinceEn)
				}
			case ffsEventSetup:
				if err := handleSetup(e.ep0, buf[off:off+8]); err != nil {
					log.Printf("ffs ep0 setup: %x: %v", buf[off:off+8], err)
				}
			}
		}
	}
}

// el2hlt is the errno f_fs.c returns after usb_ep_set_halt on ep0.
// The stall itself succeeded.
const el2hlt syscall.Errno = 51

// setupStallByRead reports which ep0 op stalls this SETUP.
// Linux f_fs.c stalls an IN request on read and an OUT request on write.
func setupStallByRead(bRequestType byte) bool {
	return bRequestType&usbDirIn != 0
}

// aoapVersion is the little-endian uint16 a phone returns for GET_PROTOCOL.
// The kernel accessory driver on this phone answers 2 while it is linked.
const (
	accGetProtocol = 51
	aoapVersion    = 2
)

// setupIsGetProtocol reports AOA GET_PROTOCOL (bmRequestType 0xC0, bRequest 51).
func setupIsGetProtocol(setup []byte) bool {
	return len(setup) >= 2 && setup[0]&usbDirIn != 0 && setup[1] == accGetProtocol
}

// handleSetup answers GET_PROTOCOL and stalls anything else.
// A Pixel's accessory driver still answers protocol version 2 after it has
// re-enumerated as 18d1:2d00. Leaving the request unanswered holds the head
// unit until its control timeout, and it never writes the version request.
func handleSetup(ep *os.File, setup []byte) error {
	if setupIsGetProtocol(setup) {
		_, err := ep.Write([]byte{byte(aoapVersion), 0})
		if err != nil {
			return fmt.Errorf("GET_PROTOCOL: %w", err)
		}
		log.Printf("ffs ep0 setup: GET_PROTOCOL -> %d", aoapVersion)
		return nil
	}
	if err := rejectSetup(ep, setup); err != nil {
		return fmt.Errorf("stall %x: %w", setup, err)
	}
	log.Printf("ffs ep0 setup: %x stalled", setup)
	return nil
}

// rejectSetup stalls a FunctionFS SETUP the MD does not implement.
// IN stalls on read and OUT stalls on write (Linux f_fs.c).
func rejectSetup(ep *os.File, setup []byte) error {
	if len(setup) < 1 {
		return fmt.Errorf("short setup")
	}
	var err error
	if setupStallByRead(setup[0]) {
		_, err = ep.Read(make([]byte, 1))
	} else {
		_, err = ep.Write([]byte{0})
	}
	if err == nil {
		return nil
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == el2hlt {
		return nil
	}
	return err
}

var _ io.ReadWriteCloser = (*Endpoints)(nil)

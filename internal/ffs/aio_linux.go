//go:build linux

package ffs

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux AIO (io_setup/io_submit/io_getevents) for FunctionFS ep1.
//
// Sync write(2) on FunctionFS holds epfile->mutex across wait_for_completion
// of the USB request (see f_fs.c ffs_epfile_io). O_NONBLOCK only affects the
// enable-wait and mutex_trylock — once a URB is queued, write(2) blocks until
// the host completes IN. That pinned our single session writer so ping_rx
// continued while pong_tx could not run (freeze log: stall n=42 = encrypted
// pong). AIO returns after usb_ep_queue and releases the mutex, so a yield
// callback can submit pongs while a media URB is still in flight.

const (
	iocbCmdPwrite = 1
	aioNR         = 64
)

type aioContext struct {
	ctx uintptr

	mu     sync.Mutex
	wait   map[uintptr]chan int64 // iocb pointer → result
	closed bool
}

type iocb struct {
	data      uint64
	key       uint32
	rwFlags   uint32
	lioOpcode uint16
	reqprio   int16
	fildes    uint32
	buf       uint64
	nbytes    uint64
	offset    int64
	reserved2 uint64
	flags     uint32
	resfd     uint32
}

type ioEvent struct {
	data uint64
	obj  uint64
	res  int64
	res2 int64
}

func aioSetup() (*aioContext, error) {
	var ctx uintptr
	_, _, errno := unix.Syscall(unix.SYS_IO_SETUP, uintptr(aioNR), uintptr(unsafe.Pointer(&ctx)), 0)
	if errno != 0 {
		return nil, errno
	}
	a := &aioContext{
		ctx:  ctx,
		wait: make(map[uintptr]chan int64),
	}
	go a.collect()
	return a, nil
}

func (a *aioContext) destroy() {
	a.mu.Lock()
	a.closed = true
	a.mu.Unlock()
	if a.ctx != 0 {
		_, _, _ = unix.Syscall(unix.SYS_IO_DESTROY, a.ctx, 0, 0)
		a.ctx = 0
	}
}

func (a *aioContext) collect() {
	var events [8]ioEvent
	for {
		a.mu.Lock()
		closed := a.closed
		ctx := a.ctx
		a.mu.Unlock()
		if closed || ctx == 0 {
			return
		}
		ts := unix.NsecToTimespec((10 * time.Millisecond).Nanoseconds())
		n, _, errno := unix.Syscall6(
			unix.SYS_IO_GETEVENTS,
			ctx,
			1, // min_nr: block for at least one (with timeout)
			uintptr(len(events)),
			uintptr(unsafe.Pointer(&events[0])),
			uintptr(unsafe.Pointer(&ts)),
			0,
		)
		if errno == unix.EINTR {
			continue
		}
		if errno != 0 {
			if errno == unix.EINVAL || errno == unix.EFAULT {
				return
			}
			continue
		}
		for i := 0; i < int(n); i++ {
			ev := events[i]
			a.mu.Lock()
			ch := a.wait[uintptr(ev.obj)]
			delete(a.wait, uintptr(ev.obj))
			a.mu.Unlock()
			if ch != nil {
				ch <- ev.res
			}
		}
	}
}

func (a *aioContext) submitPwrite(fd int, buf []byte) (*iocb, <-chan int64, error) {
	if len(buf) == 0 {
		ch := make(chan int64, 1)
		ch <- 0
		return nil, ch, nil
	}
	cb := &iocb{
		lioOpcode: iocbCmdPwrite,
		fildes:    uint32(fd),
		buf:       uint64(uintptr(unsafe.Pointer(&buf[0]))),
		nbytes:    uint64(len(buf)),
	}
	key := uintptr(unsafe.Pointer(cb))
	ch := make(chan int64, 1)
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, nil, unix.EINVAL
	}
	a.wait[key] = ch
	a.mu.Unlock()

	cbs := []*iocb{cb}
	n, _, errno := unix.Syscall(unix.SYS_IO_SUBMIT, a.ctx, 1, uintptr(unsafe.Pointer(&cbs[0])))
	if errno != 0 {
		a.mu.Lock()
		delete(a.wait, key)
		a.mu.Unlock()
		return nil, nil, errno
	}
	if int(n) != 1 {
		a.mu.Lock()
		delete(a.wait, key)
		a.mu.Unlock()
		return nil, nil, fmt.Errorf("io_submit: got %d", n)
	}
	return cb, ch, nil
}

func (e *Endpoints) initAIO() {
	ctx, err := aioSetup()
	if err != nil {
		log.Printf("ffs aio unavailable (%v) — sync writes (pongs may stall behind media)", err)
		return
	}
	e.aio = ctx
	log.Printf("ffs ep1 writes: linux AIO (mutex released while URB in flight)")
}

func (e *Endpoints) closeAIO() {
	if e.aio != nil {
		e.aio.destroy()
		e.aio = nil
	}
}

// writeAIO submits one ep1 write and waits for completion, calling yield
// between polls so the session can encrypt+submit pongs while this URB is
// outstanding. Falls back to sync write if AIO was not set up.
func (e *Endpoints) writeAIO(p []byte, yield func()) (int, error) {
	if e.aio == nil || len(p) == 0 {
		return e.writeSync(p)
	}
	buf := append([]byte(nil), p...)
	_, ch, err := e.aio.submitPwrite(int(e.epW.Fd()), buf)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EBUSY {
			return 0, ErrWriteBusy
		}
		log.Printf("ffs aio submit failed (%v) — sync fallback", err)
		return e.writeSync(p)
	}
	defer runtime.KeepAlive(buf)

	const stallLimit = 10 * time.Second
	timer := time.NewTimer(stallLimit)
	defer timer.Stop()
	for {
		if e.dead.Load() {
			return 0, ErrEndpointReset
		}
		select {
		case res := <-ch:
			if res < 0 {
				errno := unix.Errno(-res)
				if isAgain(errno) {
					return 0, ErrWriteBusy
				}
				return 0, fmt.Errorf("ffs aio write: %w", errno)
			}
			if int(res) != len(buf) {
				return int(res), fmt.Errorf("ffs aio short write %d/%d", res, len(buf))
			}
			return len(buf), nil
		case <-timer.C:
			log.Printf("ffs write stall >%v (aio n=%d) — closing endpoints", stallLimit, len(buf))
			e.fail(fmt.Errorf("%w (write stall)", ErrEndpointReset))
			_ = e.epW.Close()
			_ = e.epR.Close()
			return 0, ErrEndpointReset
		case <-time.After(500 * time.Microsecond):
			if yield != nil {
				yield()
			}
		}
	}
}

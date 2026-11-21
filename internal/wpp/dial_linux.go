//go:build linux

package wpp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func Dial(ctx context.Context, mac string) (io.ReadWriteCloser, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return nil, fmt.Errorf("wpp: daemon.peer is empty")
	}
	remote, err := parseMAC(mac)
	if err != nil {
		return nil, err
	}
	adapter, localDisp, err := findAdapter(ctx)
	if err != nil {
		return nil, err
	}
	local, err := parseMAC(localDisp)
	if err != nil {
		return nil, err
	}
	if err := requirePowered(ctx, adapter); err != nil {
		return nil, err
	}
	if err := ensureDevice(ctx, adapter, mac); err != nil {
		return nil, err
	}
	// A missing Android Auto channel is retried inside awaitChannel. Bound
	// that burst so a paired device that is not the head unit is not paged
	// once a second for the life of the process.
	wctx, wcancel := context.WithTimeout(ctx, 12*time.Second)
	defer wcancel()
	ch, err := awaitChannel(wctx, time.Second, func() (uint8, error) {
		return sdpChannel(wctx, local, remote)
	})
	if err != nil {
		return nil, err
	}
	return dialRFCOMM(ctx, local, remote, ch)
}

func findAdapter(ctx context.Context) (path, addr string, err error) {
	var last error
	for i := 0; i < 8; i++ {
		p := fmt.Sprintf("/org/bluez/hci%d", i)
		out, err := gdbus(ctx, "call", "--system", "--dest", "org.bluez", "--object-path", p,
			"--method", "org.freedesktop.DBus.Properties.Get", "org.bluez.Adapter1", "Address")
		if err != nil {
			last = err
			continue
		}
		if mac := macIn(out); mac != "" {
			return p, mac, nil
		}
	}
	if last != nil {
		return "", "", fmt.Errorf("wpp: no bluetooth adapter: %v", last)
	}
	return "", "", fmt.Errorf("wpp: no bluetooth adapter")
}

func requirePowered(ctx context.Context, adapter string) error {
	on, err := adapterPowered(ctx, adapter)
	if err != nil {
		return fmt.Errorf("wpp: bluetooth: %w", err)
	}
	if !on {
		return fmt.Errorf("wpp: bluetooth is off")
	}
	return nil
}

func adapterPowered(ctx context.Context, adapter string) (bool, error) {
	out, err := gdbus(ctx, "call", "--system", "--dest", "org.bluez", "--object-path", adapter,
		"--method", "org.freedesktop.DBus.Properties.Get", "org.bluez.Adapter1", "Powered")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "<true>"), nil
}

func ensureDevice(ctx context.Context, adapter, mac string) error {
	path := adapter + "/dev_" + devSuffix(mac)
	paired, exists, err := devicePaired(ctx, path)
	if err != nil {
		return err
	}
	if !exists {
		if err := discover(ctx, adapter, path); err != nil {
			return err
		}
		paired, exists, err = devicePaired(ctx, path)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("wpp: %s did not appear", mac)
		}
	}
	if paired {
		return nil
	}
	pctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if _, err := gdbus(pctx, "call", "--system", "--dest", "org.bluez", "--object-path", path,
		"--method", "org.bluez.Device1.Pair"); err != nil {
		return fmt.Errorf("wpp: pair %s: %w", mac, err)
	}
	return nil
}

func devicePaired(ctx context.Context, path string) (paired, exists bool, err error) {
	out, err := gdbus(ctx, "call", "--system", "--dest", "org.bluez", "--object-path", path,
		"--method", "org.freedesktop.DBus.Properties.Get", "org.bluez.Device1", "Paired")
	if err != nil {
		if unknownObject(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return strings.Contains(out, "<true>"), true, nil
}

func unknownObject(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unknownobject") || strings.Contains(s, "unknownmethod") ||
		strings.Contains(s, "does not exist") || strings.Contains(s, "doesn't exist") ||
		strings.Contains(s, "no such")
}

func discover(ctx context.Context, adapter, path string) error {
	dctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := gdbus(dctx, "call", "--system", "--dest", "org.bluez", "--object-path", adapter,
		"--method", "org.bluez.Adapter1.StartDiscovery"); err != nil && !strings.Contains(err.Error(), "InProgress") {
		return fmt.Errorf("wpp: discovery: %w", err)
	}
	defer func() {
		stop, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		_, _ = gdbus(stop, "call", "--system", "--dest", "org.bluez", "--object-path", adapter,
			"--method", "org.bluez.Adapter1.StopDiscovery")
	}()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		_, exists, err := devicePaired(dctx, path)
		if err != nil && dctx.Err() == nil {
			return err
		}
		if exists {
			return nil
		}
		select {
		case <-dctx.Done():
			return fmt.Errorf("wpp: discovery timed out")
		case <-tick.C:
		}
	}
}

func sdpChannel(ctx context.Context, local, remote [6]byte) (uint8, error) {
	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_SEQPACKET, unix.BTPROTO_L2CAP)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)
	tv := unix.NsecToTimeval((8 * time.Second).Nanoseconds())
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &tv)
	if err := unix.Bind(fd, &unix.SockaddrL2{Addr: l2Addr(local)}); err != nil {
		return 0, fmt.Errorf("wpp: sdp bind: %w", err)
	}
	if err := unix.Connect(fd, &unix.SockaddrL2{PSM: 1, Addr: l2Addr(remote)}); err != nil {
		return 0, fmt.Errorf("wpp: sdp connect: %w", err)
	}
	req := sdpSearch()
	if _, err := unix.Write(fd, req); err != nil {
		return 0, fmt.Errorf("wpp: sdp write: %w", err)
	}
	buf := make([]byte, 512)
	n, err := unix.Read(fd, buf)
	if err != nil {
		return 0, fmt.Errorf("wpp: sdp read: %w", err)
	}
	if ch, ok := channelFromSDP(buf[:n]); ok {
		return ch, nil
	}
	return 0, ErrNoChannel
}

func dialRFCOMM(ctx context.Context, local, remote [6]byte, channel uint8) (io.ReadWriteCloser, error) {
	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_STREAM, unix.BTPROTO_RFCOMM)
	if err != nil {
		return nil, err
	}
	wait := 15 * time.Second
	if d, ok := ctx.Deadline(); ok {
		if left := time.Until(d); left > 0 {
			wait = left
		}
	}
	tv := unix.NsecToTimeval(wait.Nanoseconds())
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &tv)
	if err := unix.Bind(fd, &unix.SockaddrRFCOMM{Addr: local}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("wpp: rfcomm bind: %w", err)
	}
	if err := unix.Connect(fd, &unix.SockaddrRFCOMM{Addr: remote, Channel: channel}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("wpp: rfcomm connect: %w", err)
	}
	zero := unix.Timeval{}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &zero)
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &zero)
	return os.NewFile(uintptr(fd), "rfcomm"), nil
}

func gdbus(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gdbus", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			return out, err
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}

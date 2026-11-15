//go:build linux

package usbcable

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// pauseOffline waits until path may have changed, or stop is closed.
// poll watches POLLPRI, which is what sysfs_notify wakes. A regular file
// or a sysfs node that returns immediately falls back to offlineRecheck
// so a level-triggered poll cannot spin.
func pauseOffline(path string, stop <-chan struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		return sleepStop(stop, offlineRecheck)
	}
	defer f.Close()
	buf := make([]byte, 512)
	_, _ = f.Read(buf)
	_, _ = f.Seek(0, 0)

	pr, pw, err := os.Pipe()
	if err != nil {
		return sleepStop(stop, offlineRecheck)
	}
	defer pr.Close()
	defer pw.Close()
	done := make(chan struct{})
	defer close(done)
	if stop != nil {
		go func() {
			select {
			case <-stop:
				_, _ = pw.Write([]byte{1})
			case <-done:
			}
		}()
	}

	fds := []unix.PollFd{
		{Fd: int32(f.Fd()), Events: unix.POLLPRI | unix.POLLERR},
		{Fd: int32(pr.Fd()), Events: unix.POLLIN},
	}
	t0 := time.Now()
	n, _ := unix.Poll(fds, int(offlineRecheck/time.Millisecond))
	if stop != nil {
		select {
		case <-stop:
			return ErrStopped
		default:
		}
	}
	if n > 0 && time.Since(t0) < 50*time.Millisecond {
		return sleepStop(stop, offlineRecheck)
	}
	return nil
}

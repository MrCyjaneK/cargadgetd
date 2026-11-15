//go:build linux

package aoa

import (
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/cargadgetd/cargadgetd/internal/stopwait"
	"github.com/cargadgetd/cargadgetd/internal/usbcable"
)

const devPath = "/dev/usb_accessory"

// _IOW('M', nr, char[256]) and _IO('M', 7). Matches f_accessory.h.
const (
	ioctlGetManufacturer = 0x41004d01
	ioctlGetModel        = 0x41004d02
	ioctlGetDescription  = 0x41004d03
	ioctlGetVersion      = 0x41004d04
	ioctlGetURI          = 0x41004d05
	ioctlGetSerial       = 0x41004d06
	ioctlIsStart         = 0x00004d07
)

// Wait blocks until the host completes AOA START for Android Auto.
// A start that is already pending when Wait is called is ignored. Unplug
// leaves that flag set, and treating it as a new host rebinds the accessory
// before usb-moded can show the ask dialog. GET_PROTOCOL clears the flag,
// and the host sends it before START, which is the edge this waits for.
//
// With the cable out this blocks in poll on VBUS and does not read the
// accessory device. The 2ms sample runs only while a stale start is latched.
func Wait(stop <-chan struct{}) (Identity, error) {
	var prev int
	primed := false
	stale := false
	cableOff := false
	var fastUntil time.Time
	for {
		if !usbcable.Online() {
			fastUntil = time.Time{}
			if !cableOff {
				log.Printf("aoa: no USB host; sleeping until the cable is connected")
				cableOff = true
			}
			if err := usbcable.WaitOnline(stop); err != nil {
				return Identity{}, ErrStopped
			}
			log.Printf("aoa: USB host present")
			cableOff = false
			fastUntil = time.Now().Add(fastSampleWindow)
		}
		f, err := os.OpenFile(devPath, os.O_RDWR, 0)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("aoa: open %s: %v", devPath, err)
			}
		} else {
			cur, ierr := ioctlStart(f)
			if ierr != nil {
				f.Close()
				log.Printf("aoa: start flag: %v", ierr)
			} else if !primed {
				prev = cur
				primed = true
				stale = cur == 1
				if stale {
					log.Printf("aoa: stale start already pending; waiting for the next host exchange")
				}
				f.Close()
			} else if prev == 0 && cur == 1 {
				id, rerr := readIdentity(f)
				f.Close()
				prev = cur
				stale = true
				if rerr != nil {
					return Identity{}, rerr
				}
				if IsAndroidAuto(id.Model) {
					return id, nil
				}
				log.Printf("aoa: start for model %q is not Android Auto", id.Model)
			} else {
				f.Close()
				stale = cur == 1
				prev = cur
			}
		}
		if stopwait.Sleep(stop, sampleGapAt(stale, time.Now(), fastUntil)) {
			return Identity{}, ErrStopped
		}
	}
}

func ioctlStart(f *os.File) (int, error) {
	r, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), ioctlIsStart, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}

func readIdentity(f *os.File) (Identity, error) {
	var id Identity
	var err error
	if id.Manufacturer, err = ioctlString(f, ioctlGetManufacturer); err != nil {
		return id, fmt.Errorf("manufacturer: %w", err)
	}
	if id.Model, err = ioctlString(f, ioctlGetModel); err != nil {
		return id, fmt.Errorf("model: %w", err)
	}
	if id.Description, err = ioctlString(f, ioctlGetDescription); err != nil {
		return id, fmt.Errorf("description: %w", err)
	}
	if id.Version, err = ioctlString(f, ioctlGetVersion); err != nil {
		return id, fmt.Errorf("version: %w", err)
	}
	if id.URI, err = ioctlString(f, ioctlGetURI); err != nil {
		return id, fmt.Errorf("uri: %w", err)
	}
	if id.Serial, err = ioctlString(f, ioctlGetSerial); err != nil {
		return id, fmt.Errorf("serial: %w", err)
	}
	return id, nil
}

func ioctlString(f *os.File, req uintptr) (string, error) {
	buf := make([]byte, 256)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return "", errno
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(buf[:n]), nil
}

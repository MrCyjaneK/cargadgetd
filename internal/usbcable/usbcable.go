// Package usbcable reports USB VBUS / cable presence from sysfs.
package usbcable

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/stopwait"
)

// Paths checked in order. First readable file wins.
var onlineFiles = []string{
	"/sys/class/power_supply/usb/online",
}

var extconFiles = []string{
	"/sys/class/extcon/extcon0/state",
	"/sys/class/extcon/extcon1/state",
}

const poll = 500 * time.Millisecond

// offlineRecheck is how often to re-read VBUS when poll() does not wake.
// The sysfs node notifies on change; this is only the missed-event fallback.
var offlineRecheck = time.Second

// ErrStopped is returned when a wait is cancelled.
var ErrStopped = errors.New("usbcable: stopped")

// confirmOffline is how many consecutive VBUS=0 polls after seeing VBUS=1
// count as unplug. AOA accessory-mode switch dips online for ~1s on musb-sprd
// (C2); a 1s window tore the gadget down before DHU could send a version request.
var confirmOffline = 6

// Probe returns cable presence plus the sysfs file and raw value used.
// Missing sysfs → online=true (desktop/tcp must not tear down).
func Probe() (online bool, path, raw string) {
	for _, p := range onlineFiles {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		raw = strings.TrimSpace(string(b))
		return raw == "1", p, raw
	}
	for _, p := range extconFiles {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		raw = strings.TrimSpace(string(b))
		for _, line := range strings.Split(raw, "\n") {
			if strings.TrimSpace(line) == "USB=1" {
				return true, p, raw
			}
		}
		return false, p, raw
	}
	return true, "", "(no sysfs)"
}

// Online reports whether a USB cable is connected.
func Online() bool {
	on, _, _ := Probe()
	return on
}

// WaitOnline blocks until a USB host is present. With no sysfs it returns
// immediately (desktop/tcp must not wait forever). stop cancels the wait.
// The wait is poll() on the VBUS node, so an unplugged phone does not arm
// a short timer.
func WaitOnline(stop <-chan struct{}) error {
	for {
		if err := sleepStop(stop, 0); err != nil {
			return err
		}
		on, path, _ := Probe()
		if on || path == "" {
			return nil
		}
		if err := pauseOffline(path, stop); err != nil {
			return err
		}
	}
}

func sleepStop(stop <-chan struct{}, d time.Duration) error {
	if stopwait.Sleep(stop, d) {
		return ErrStopped
	}
	return nil
}

// WaitDisconnect blocks until the cable has been seen connected and then
// stays disconnected for confirmOffline poll intervals. stop cancels the wait.
func WaitDisconnect(stop <-chan struct{}) {
	seenOn := false
	offline := 0
	need := confirmOffline
	if need < 2 {
		need = 2
	}
	for {
		if stop != nil {
			select {
			case <-stop:
				return
			default:
			}
		}
		on, src, raw := Probe()
		if on {
			if !seenOn {
				log.Printf("usbcable: VBUS on src=%s raw=%q", src, raw)
			} else if offline > 0 {
				log.Printf("usbcable: VBUS returned after %d off poll(s) src=%s raw=%q", offline, src, raw)
			}
			seenOn = true
			offline = 0
		} else if seenOn {
			offline++
			log.Printf("usbcable: VBUS off poll=%d/%d src=%s raw=%q", offline, need, src, raw)
			if offline >= need {
				log.Printf("usbcable: disconnect confirmed src=%s raw=%q", src, raw)
				return
			}
		}
		select {
		case <-stop:
			return
		case <-time.After(poll):
		}
	}
}

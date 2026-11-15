// Package aoa watches the kernel Android Open Accessory device.
//
// On the Jolla C2 the gadget function answers GET_PROTOCOL, stores the six
// SEND_STRING values, and sets start-requested on START. It does not change
// idProduct. Userspace has to re-enumerate as 18d1:2d00 after that.
package aoa

import (
	"errors"
	"time"
)

// ErrStopped is returned when Wait is cancelled before an Android Auto start.
var ErrStopped = errors.New("aoa: stopped")

const (
	// idleSample is the gap between START-flag reads while the flag is 0
	// and the fast window has elapsed. START latches, so this cannot miss
	// a host. 250ms still lets the CPU enter deep idle when a charger sits
	// in for hours; the old 2ms poll did not.
	idleSample = 250 * time.Millisecond
	// edgeSample is used while a stale START is latched, and for the first
	// few seconds after VBUS appears. GET_PROTOCOL clears the flag and
	// START sets it again a few milliseconds later. A slow sample can miss
	// the 0 in between, and a car gives up if the phone is still the ask
	// gadget when START returns.
	edgeSample = 2 * time.Millisecond
	// fastSampleWindow is how long after VBUS to poll at edgeSample.
	// Head units send GET_PROTOCOL and START within a couple of seconds
	// of enumeration. After that a cable is usually just charging.
	fastSampleWindow = 5 * time.Second
)

// sampleGapAt is how long to sleep before the next ACCESSORY_IS_START_REQUESTED
// read. stale is true when the previous sample was already 1. A non-zero
// fastUntil keeps the 2ms sample until that time so START disconnects promptly.
func sampleGapAt(stale bool, now, fastUntil time.Time) time.Duration {
	if stale || (!fastUntil.IsZero() && now.Before(fastUntil)) {
		return edgeSample
	}
	return idleSample
}

// Identity is the six strings from the most recent AOA exchange.
type Identity struct {
	Manufacturer string
	Model        string
	Description  string
	Version      string
	URI          string
	Serial       string
}

// IsAndroidAuto reports whether the host asked for Android Auto projection.
// Desktop Head Unit sends the long model name. Other head units send "Android Auto".
func IsAndroidAuto(model string) bool {
	switch model {
	case "Android Open Automotive Protocol", "Android Auto":
		return true
	default:
		return false
	}
}

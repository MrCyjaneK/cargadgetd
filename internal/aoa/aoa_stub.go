//go:build !linux

package aoa

import "fmt"

// Wait is only implemented on Linux, where /dev/usb_accessory exists.
func Wait(stop <-chan struct{}) (Identity, error) {
	if stop == nil {
		return Identity{}, fmt.Errorf("aoa: not supported on this OS")
	}
	<-stop
	return Identity{}, ErrStopped
}

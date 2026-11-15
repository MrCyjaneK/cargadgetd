//go:build !linux

package usbcable

func pauseOffline(path string, stop <-chan struct{}) error {
	return sleepStop(stop, offlineRecheck)
}

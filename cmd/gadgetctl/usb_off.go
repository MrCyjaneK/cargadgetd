package main

import (
	"os"

	"github.com/cargadgetd/cargadgetd/internal/usbgadget"
)

// Hooks so tests can restore a fake gadget without systemctl or pgrep.
var (
	stopGadgetAgents = stopRunningAgents
	unitActive       = serviceActive
	runSystemctl     = systemctl
)

// usbOff stops a running gadget agent, puts a borrowed g1 back from the snap,
// and starts usb-moded when it is not already active. A second call with no
// snap is success.
func usbOff(ffsDir string) error {
	if err := stopGadgetAgents(); err != nil {
		return err
	}
	snap, err := loadGadgetSnap()
	if err != nil {
		return err
	}
	if snap != nil {
		restoreAdopted(snap, ffsDir)
	}
	_ = os.Remove(usbgadget.AdoptedFile)
	_ = os.Remove(gadgetSnapFile)
	if unitActive("usb-moded") || unitActive("usb_moded") {
		return nil
	}
	if err := runSystemctl("start", "usb-moded"); err == nil {
		return nil
	}
	return runSystemctl("start", "usb_moded")
}

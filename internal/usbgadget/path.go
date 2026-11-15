// Package usbgadget is the configfs path gadgetctl publishes when it
// borrows usb-moded's gadget instead of creating one of its own.
package usbgadget

import (
	"os"
	"path/filepath"
	"strings"
)

// AdoptedFile is a single line: the configfs gadget directory in use.
var AdoptedFile = "/run/cargadgetd/gadget"

// UDCPath is <gadget>/UDC. AdoptedFile wins over the configured aa path.
func UDCPath(configured string) string {
	if b, err := os.ReadFile(AdoptedFile); err == nil {
		if p := strings.TrimSpace(string(b)); p != "" {
			return filepath.Join(p, "UDC")
		}
	}
	return filepath.Join(configured, "UDC")
}

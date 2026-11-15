package usbgadget

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUDCPath(t *testing.T) {
	dir := t.TempDir()
	AdoptedFile = filepath.Join(dir, "gadget")
	t.Cleanup(func() { AdoptedFile = "/run/cargadgetd/gadget" })

	configured := "/sys/kernel/config/usb_gadget/aa"
	if got := UDCPath(configured); got != configured+"/UDC" {
		t.Fatalf("got %s", got)
	}
	if err := os.WriteFile(AdoptedFile, []byte("/sys/kernel/config/usb_gadget/g1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := UDCPath(configured); got != "/sys/kernel/config/usb_gadget/g1/UDC" {
		t.Fatalf("got %s", got)
	}
}

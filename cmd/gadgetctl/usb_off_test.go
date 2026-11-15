package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/usbgadget"
)

func TestUSBOffRestoresSnapTwice(t *testing.T) {
	gadgetUnbindSettle = 0
	outside := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	dir := t.TempDir()
	usbgadget.AdoptedFile = filepath.Join(dir, "gadget")
	gadgetSnapFile = filepath.Join(dir, "gadget.snap")
	t.Cleanup(func() {
		usbgadget.AdoptedFile = "/run/cargadgetd/gadget"
		gadgetSnapFile = "/run/cargadgetd/gadget.snap"
	})

	prevStop, prevActive, prevCtl := stopGadgetAgents, unitActive, runSystemctl
	var calls []string
	active := false
	stopGadgetAgents = func() error { return nil }
	unitActive = func(unit string) bool {
		return active && (unit == "usb-moded" || unit == "usb_moded")
	}
	runSystemctl = func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "start" {
			active = true
		}
		return nil
	}
	t.Cleanup(func() {
		stopGadgetAgents, unitActive, runSystemctl = prevStop, prevActive, prevCtl
	})

	g := filepath.Join(dir, "g1")
	cfgDir := filepath.Join(g, "configs", "b.1")
	for _, d := range []string{cfgDir, filepath.Join(g, "functions", "mass_storage.usb0"), filepath.Join(g, "strings", "0x409")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(g, rel), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("UDC", "\n")
	write("idVendor", "0x0e8d\n")
	write("idProduct", "0x0afe\n")
	write("bcdDevice", "0x0200\n")
	write("bcdUSB", "0x0200\n")
	write("strings/0x409/serialnumber", "jolla\n")
	write("strings/0x409/manufacturer", "Jolla\n")
	write("strings/0x409/product", "Phone\n")
	if err := os.Symlink("../../functions/mass_storage.usb0", filepath.Join(cfgDir, "mass_storage.usb0")); err != nil {
		t.Fatal(err)
	}
	snap, err := captureGadget(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveGadgetSnap(snap); err != nil {
		t.Fatal(err)
	}
	if err := publishAdoptedGadget(g); err != nil {
		t.Fatal(err)
	}
	if err := unlinkConfigLinks(g, snap.Configs); err != nil {
		t.Fatal(err)
	}
	if err := writeAccessoryIdentity(g); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(g, "functions", "ffs.aa"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../functions/ffs.aa", filepath.Join(cfgDir, "ffs.aa")); err != nil {
		t.Fatal(err)
	}

	ffsDir := filepath.Join(dir, "ffs")
	if err := os.MkdirAll(ffsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := usbOff(ffsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gadgetSnapFile); !os.IsNotExist(err) {
		t.Fatalf("snap still present: %v", err)
	}
	if _, err := os.Stat(usbgadget.AdoptedFile); !os.IsNotExist(err) {
		t.Fatalf("adopted path still present: %v", err)
	}
	target, err := os.Readlink(filepath.Join(cfgDir, "mass_storage.usb0"))
	if err != nil || target != "../../functions/mass_storage.usb0" {
		t.Fatalf("mass_storage link %q err=%v", target, err)
	}
	if _, err := os.Lstat(filepath.Join(cfgDir, "ffs.aa")); !os.IsNotExist(err) {
		t.Fatalf("ffs.aa still linked: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "idVendor")); err != nil || string(b) != "0x0e8d\n" {
		t.Fatalf("idVendor=%q err=%v", b, err)
	}
	if len(calls) != 1 || calls[0] != "start usb-moded" {
		t.Fatalf("systemctl calls %v", calls)
	}

	if err := usbOff(ffsDir); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("second off started usb-moded again: %v", calls)
	}
	if _, err := os.Stat(g); err != nil {
		t.Fatal(err)
	}
}

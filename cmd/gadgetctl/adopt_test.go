package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/usbgadget"
)

func TestSelectAdoptedGadget(t *testing.T) {
	root := t.TempDir()
	class := filepath.Join(root, "android_usb")
	gadgets := filepath.Join(root, "usb_gadget")
	if err := os.Mkdir(class, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gadgets, "g1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gadgets, "aa"), 0755); err != nil {
		t.Fatal(err)
	}
	androidUSBClassPath = class
	gadgetConfigRoot = gadgets
	t.Cleanup(func() {
		androidUSBClassPath = "/sys/class/android_usb"
		gadgetConfigRoot = "/sys/kernel/config/usb_gadget"
	})

	got, ok := selectAdoptedGadget(filepath.Join(gadgets, "aa"))
	if !ok || filepath.Base(got) != "g1" {
		t.Fatalf("got %q ok=%v", got, ok)
	}

	androidUSBClassPath = filepath.Join(root, "missing")
	if _, ok := selectAdoptedGadget(filepath.Join(gadgets, "aa")); ok {
		t.Fatal("adopted without android_usb class")
	}
}

func TestDropNonFFSLinks(t *testing.T) {
	g := t.TempDir()
	cfg := filepath.Join(g, "configs", "b.1")
	for _, d := range []string{
		cfg,
		filepath.Join(g, "functions", "ffs.aa"),
		filepath.Join(g, "functions", "accessory.gs2"),
		filepath.Join(g, "functions", "mass_storage.usb0"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"ffs.aa", "accessory.gs2", "mass_storage.usb0"} {
		if err := os.Symlink("../../functions/"+name, filepath.Join(cfg, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfg, "MaxPower"), []byte("500\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := dropNonFFSLinks(g); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(cfg, "ffs.aa")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(cfg, "accessory.gs2")); !os.IsNotExist(err) {
		t.Fatalf("accessory.gs2 still linked: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg, "mass_storage.usb0")); !os.IsNotExist(err) {
		t.Fatalf("mass_storage.usb0 still linked: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(cfg, "MaxPower")); err != nil || string(b) != "500\n" {
		t.Fatalf("MaxPower=%q err=%v", b, err)
	}
}

func TestRestoreAdoptedLeavesForeignNodes(t *testing.T) {
	gadgetUnbindSettle = 0
	// configfs looks symlink targets up from cwd. Stay outside the gadget
	// tree so a relink that forgets to chdir cannot see the function dir.
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
	root := dir
	g := filepath.Join(root, "g1")
	cfgDir := filepath.Join(g, "configs", "b.1")
	fnDir := filepath.Join(g, "functions", "mass_storage.usb0")
	for _, d := range []string{cfgDir, fnDir, filepath.Join(g, "os_desc"), filepath.Join(g, "webusb"), filepath.Join(g, "strings", "0x409")} {
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
	write("configs/b.1/MaxPower", "500\n")
	if err := os.Symlink("../../functions/mass_storage.usb0", filepath.Join(cfgDir, "mass_storage.usb0")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../configs/b.1", filepath.Join(g, "os_desc", "b.1")); err != nil {
		t.Fatal(err)
	}

	snap, err := captureGadget(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := unlinkConfigLinks(g, snap.Configs); err != nil {
		t.Fatal(err)
	}
	if err := writeAccessoryIdentity(g); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "idVendor")); err != nil || string(b) != "0x18d1\n" {
		t.Fatalf("switched vendor=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "idProduct")); err != nil || string(b) != "0x2d00\n" {
		t.Fatalf("switched product id=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "strings/0x409/product")); err != nil || string(b) != "Phone\n" {
		t.Fatalf("product overwritten: %q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "strings/0x409/manufacturer")); err != nil || string(b) != "Jolla\n" {
		t.Fatalf("manufacturer overwritten: %q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "bcdDevice")); err != nil || string(b) != "0x0200\n" {
		t.Fatalf("bcdDevice overwritten: %q err=%v", b, err)
	}
	if err := os.Mkdir(filepath.Join(g, "functions", "ffs.aa"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := linkFFSFunction(g, snap); err != nil {
		t.Fatal(err)
	}

	ffsDir := filepath.Join(root, "ffs")
	if err := os.MkdirAll(ffsDir, 0755); err != nil {
		t.Fatal(err)
	}
	restoreAdopted(snap, ffsDir)

	if b, err := os.ReadFile(filepath.Join(g, "idVendor")); err != nil || string(b) != "0x0e8d\n" {
		t.Fatalf("idVendor=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "strings/0x409/product")); err != nil || string(b) != "Phone\n" {
		t.Fatalf("product=%q err=%v", b, err)
	}
	target, err := os.Readlink(filepath.Join(cfgDir, "mass_storage.usb0"))
	if err != nil || target != "../../functions/mass_storage.usb0" {
		t.Fatalf("mass_storage link %q err=%v", target, err)
	}
	if _, err := os.Lstat(filepath.Join(cfgDir, "ffs.aa")); !os.IsNotExist(err) {
		t.Fatalf("ffs.aa link still present: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(g, "functions", "ffs.aa")); !os.IsNotExist(err) {
		t.Fatalf("ffs.aa function still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(g, "functions", "mass_storage.usb0")); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(cfgDir, "MaxPower")); err != nil || string(b) != "500\n" {
		t.Fatalf("MaxPower=%q err=%v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(g, "os_desc", "b.1")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(g, "webusb")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(g); err != nil {
		t.Fatal(err)
	}
}

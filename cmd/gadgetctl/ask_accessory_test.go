package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkAskAccessory(t *testing.T) {
	gadgetUnbindSettle = 0
	root := t.TempDir()
	gadgetConfigRoot = filepath.Join(root, "usb_gadget")
	t.Cleanup(func() {
		gadgetConfigRoot = "/sys/kernel/config/usb_gadget"
	})
	g := filepath.Join(gadgetConfigRoot, "g1")
	cfg := filepath.Join(g, "configs", "b.1")
	if err := os.MkdirAll(cfg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(g, "functions", "accessory.gs2"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(g, "functions", "mass_storage.usb0"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(g, rel), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("UDC", "11201000.usb0\n")
	write("idVendor", "0x0e8d\n")
	write("idProduct", "0x0afe\n")
	write("bMaxPacketSize0", "0x00\n")
	if err := os.Symlink("../../functions/mass_storage.usb0", filepath.Join(cfg, "mass_storage.usb0")); err != nil {
		t.Fatal(err)
	}

	n, err := linkAskAccessory()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("linked %d", n)
	}
	target, err := os.Readlink(filepath.Join(cfg, "accessory.gs2"))
	if err != nil || target != filepath.Join("functions", "accessory.gs2") {
		t.Fatalf("link %q err=%v", target, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "UDC")); err != nil || string(b) != "11201000.usb0\n" {
		t.Fatalf("UDC=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "bMaxPacketSize0")); err != nil || string(b) != "0x40\n" {
		t.Fatalf("pkt=%q err=%v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "idVendor")); err != nil || string(b) != "0x0e8d\n" {
		t.Fatalf("vendor=%q err=%v", b, err)
	}

	n, err = linkAskAccessory()
	if err != nil || n != 0 {
		t.Fatalf("second link n=%d err=%v", n, err)
	}

	// usb-moded drops the symlink when it applies a mode on cable insert.
	if err := os.Remove(filepath.Join(cfg, "accessory.gs2")); err != nil {
		t.Fatal(err)
	}
	n, err = linkAskAccessory()
	if err != nil || n != 1 {
		t.Fatalf("relink after drop n=%d err=%v", n, err)
	}
}

func TestLinkAskAccessoryBindsUnbound(t *testing.T) {
	gadgetUnbindSettle = 0
	root := t.TempDir()
	gadgetConfigRoot = filepath.Join(root, "usb_gadget")
	udcClassPath = filepath.Join(root, "udc")
	t.Cleanup(func() {
		gadgetConfigRoot = "/sys/kernel/config/usb_gadget"
		udcClassPath = "/sys/class/udc"
	})
	if err := os.MkdirAll(udcClassPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(udcClassPath, "11201000.usb0"), []byte("\n"), 0644); err != nil {
		t.Fatal(err)
	}
	g := filepath.Join(gadgetConfigRoot, "g1")
	cfg := filepath.Join(g, "configs", "b.1")
	if err := os.MkdirAll(cfg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(g, "functions", "accessory.gs2"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "UDC"), []byte("\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "idVendor"), []byte("0x0e8d\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "bMaxPacketSize0"), []byte("0x40\n"), 0644); err != nil {
		t.Fatal(err)
	}
	n, err := linkAskAccessory()
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if b, err := os.ReadFile(filepath.Join(g, "UDC")); err != nil || string(b) != "11201000.usb0\n" {
		t.Fatalf("UDC=%q err=%v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(cfg, "accessory.gs2")); err != nil {
		t.Fatal(err)
	}
}

func TestLinkAskAccessorySkipsGoogleVendor(t *testing.T) {
	gadgetUnbindSettle = 0
	root := t.TempDir()
	gadgetConfigRoot = filepath.Join(root, "usb_gadget")
	t.Cleanup(func() {
		gadgetConfigRoot = "/sys/kernel/config/usb_gadget"
	})
	g := filepath.Join(gadgetConfigRoot, "g1")
	if err := os.MkdirAll(filepath.Join(g, "configs", "b.1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(g, "functions", "accessory.gs2"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "UDC"), []byte("musb-hdrc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "idVendor"), []byte("0x18d1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	n, err := linkAskAccessory()
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Lstat(filepath.Join(g, "configs", "b.1", "accessory.gs2")); !os.IsNotExist(err) {
		t.Fatalf("c2 gadget was modified: %v", err)
	}
}

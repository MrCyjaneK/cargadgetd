package wpp

import (
	"bytes"
	"io"
	"testing"
)

func TestBluezWakeIgnoresRSSI(t *testing.T) {
	dev := "dev_DC_56_7B_68_B1_28"
	rssi := append([]byte("/org/bluez/hci1/"+dev), 0)
	rssi = append(rssi, dbusString("RSSI")...)
	if bluezWake(rssi, dev) {
		t.Fatal("rssi")
	}
	if !bluezWake(dbusString("Powered"), dev) {
		t.Fatal("powered")
	}
	conn := append(append([]byte("/org/bluez/hci1/"+dev), 0), dbusString("Connected")...)
	if !bluezWake(conn, dev) {
		t.Fatal("connected")
	}
	other := append(append([]byte("/org/bluez/hci1/dev_11_22_33_44_55_66"), 0), dbusString("Connected")...)
	if bluezWake(other, dev) {
		t.Fatal("other device")
	}
	added := append(append([]byte("InterfacesAdded"), 0), append([]byte("/org/bluez/hci1/"+dev), 0)...)
	if !bluezWake(added, dev) {
		t.Fatal("added")
	}
}

func TestMethodCallFraming(t *testing.T) {
	msg := methodCall(1, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "Hello", "", "")
	if msg[0] != 'l' || msg[1] != 1 || msg[3] != 1 {
		t.Fatalf("hdr %x", msg[:4])
	}
	if !bytes.Contains(msg, dbusString("Hello")) {
		t.Fatalf("hello missing %x", msg)
	}
	if len(msg)%8 != 0 {
		t.Fatalf("hello len %d", len(msg))
	}
	got, err := readMsg(bytes.NewReader(msg))
	if err != nil || !bytes.Equal(got, msg) {
		t.Fatalf("hello roundtrip %d/%d %v", len(got), len(msg), err)
	}
	with := methodCall(2, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "AddMatch", "s", "type='signal',sender='org.bluez'")
	got, err = readMsg(bytes.NewReader(with))
	if err != nil || !bytes.Equal(got, with) {
		t.Fatalf("match roundtrip %d/%d %v", len(got), len(with), err)
	}
	if !bytes.Contains(with, dbusString("type='signal',sender='org.bluez'")) {
		t.Fatal("rule missing")
	}
	// A second message must not be consumed by the first read.
	both := append(append([]byte{}, msg...), with...)
	r := bytes.NewReader(both)
	first, err := readMsg(r)
	if err != nil || !bytes.Equal(first, msg) {
		t.Fatalf("first %v", err)
	}
	second, err := readMsg(r)
	if err != nil || !bytes.Equal(second, with) {
		t.Fatalf("second %v", err)
	}
	if _, err := readMsg(r); err != io.EOF {
		t.Fatal(err)
	}
}

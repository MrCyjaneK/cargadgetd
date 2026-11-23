package main

import "testing"

func TestEffectiveListen(t *testing.T) {
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "usb"); got != "ffs:///dev/usb-ffs/aa" {
		t.Fatalf("usb ffs: %s", got)
	}
	if got := effectiveListen("tcp://127.0.0.1:5277", "usb"); got != "ffs:///dev/usb-ffs/aa" {
		t.Fatalf("usb old default: %s", got)
	}
	if got := effectiveListen("", "usb"); got != "ffs:///dev/usb-ffs/aa" {
		t.Fatalf("usb empty: %s", got)
	}
	if got := effectiveListen("tcp://10.0.0.2:9", "usb"); got != "tcp://10.0.0.2:9" {
		t.Fatalf("usb custom: %s", got)
	}
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "wireless"); got != "tcp://127.0.0.1:5277" {
		t.Fatalf("wireless ffs: %s", got)
	}
	if got := effectiveListen("tcp://10.0.0.2:9", "wireless"); got != "tcp://10.0.0.2:9" {
		t.Fatalf("wireless custom: %s", got)
	}
	if got := effectiveListen("tcp://127.0.0.1:5277", "wpp"); got != "tcp://127.0.0.1:5277" {
		t.Fatalf("wpp tcp: %s", got)
	}
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "wpp"); got != "tcp://127.0.0.1:5277" {
		t.Fatalf("wpp ffs: %s", got)
	}
}

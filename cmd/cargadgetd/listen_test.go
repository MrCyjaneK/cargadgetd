package main

import "testing"

func TestEffectiveListenSkipsFFSWhenWireless(t *testing.T) {
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "tcp://127.0.0.1:5277", "wireless"); got != "tcp://127.0.0.1:5277" {
		t.Fatalf("configured tcp: %s", got)
	}
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "ffs:///dev/usb-ffs/aa", "wireless"); got != "tcp://127.0.0.1:5277" {
		t.Fatalf("both ffs: %s", got)
	}
	if got := effectiveListen("ffs:///dev/usb-ffs/aa", "tcp://127.0.0.1:5277", "usb"); got != "ffs:///dev/usb-ffs/aa" {
		t.Fatalf("usb keeps ffs: %s", got)
	}
	if got := effectiveListen("tcp://10.0.0.2:9", "ffs:///dev/usb-ffs/aa", "wireless"); got != "tcp://10.0.0.2:9" {
		t.Fatalf("explicit tcp: %s", got)
	}
}

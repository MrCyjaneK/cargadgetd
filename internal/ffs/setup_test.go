package ffs

import "testing"

func TestSetupStallDirection(t *testing.T) {
	if !setupStallByRead(0xC0) {
		t.Fatal("GET_PROTOCOL is IN and stalls on read")
	}
	if setupStallByRead(0x40) {
		t.Fatal("SEND_STRING is OUT and stalls on write")
	}
}

func TestSetupIsGetProtocol(t *testing.T) {
	if !setupIsGetProtocol([]byte{0xC0, 51, 0, 0, 0, 0, 2, 0}) {
		t.Fatal("GET_PROTOCOL")
	}
	if setupIsGetProtocol([]byte{0x40, 51}) {
		t.Fatal("OUT is not GET_PROTOCOL")
	}
	if setupIsGetProtocol([]byte{0xC0, 52}) {
		t.Fatal("SEND_STRING")
	}
}

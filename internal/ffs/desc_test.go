package ffs

import (
	"encoding/binary"
	"testing"
)

func TestAccessoryDescriptorsLayout(t *testing.T) {
	d := buildAccessoryDescriptors()
	if len(d) < 16 {
		t.Fatalf("too short: %d", len(d))
	}
	if got := binary.LittleEndian.Uint32(d[0:4]); got != functionfsDescriptorsMagicV2 {
		t.Fatalf("magic=%d want %d (legacy hex 0x402c4491 is wrong for modern kernels)", got, functionfsDescriptorsMagicV2)
	}
	if got := binary.LittleEndian.Uint32(d[4:8]); got != uint32(len(d)) {
		t.Fatalf("length field=%d want %d", got, len(d))
	}
	if got := binary.LittleEndian.Uint32(d[8:12]); got != functionfsHasFSDesc|functionfsHasHSDesc {
		t.Fatalf("flags=%d", got)
	}
	s := buildStrings()
	if got := binary.LittleEndian.Uint32(s[0:4]); got != functionfsStringsMagic {
		t.Fatalf("strings magic=%d want %d", got, functionfsStringsMagic)
	}
	if got := binary.LittleEndian.Uint32(s[4:8]); got != uint32(len(s)) {
		t.Fatalf("strings length=%d want %d", got, len(s))
	}
}

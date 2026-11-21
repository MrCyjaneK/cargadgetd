package wpp

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseMACWireOrder(t *testing.T) {
	wire, err := parseMAC("CC:BB:AA:33:22:11")
	if err != nil {
		t.Fatal(err)
	}
	want := [6]byte{0x11, 0x22, 0x33, 0xaa, 0xbb, 0xcc}
	if wire != want {
		t.Fatalf("%x", wire)
	}
	if devSuffix("cc:bb:aa:33:22:11") != "CC_BB_AA_33_22_11" {
		t.Fatal(devSuffix("cc:bb:aa:33:22:11"))
	}
	if _, err := parseMAC("nope"); err == nil {
		t.Fatal("accepted")
	}
	if l2Addr(wire) != ([6]byte{0xcc, 0xbb, 0xaa, 0x33, 0x22, 0x11}) {
		t.Fatalf("l2 %x", l2Addr(wire))
	}
}

func TestSDPSearchNamesTheService(t *testing.T) {
	pkt := sdpSearch()
	if !bytes.Contains(pkt, aaUUID[:]) {
		t.Fatalf("uuid missing: %x", pkt)
	}
	if binaryLen(pkt) != len(pkt)-5 {
		t.Fatalf("param length %d pkt %d", binaryLen(pkt), len(pkt))
	}
}

func binaryLen(pkt []byte) int {
	return int(pkt[3])<<8 | int(pkt[4])
}

func TestChannelFromSDP(t *testing.T) {
	// ProtocolDescriptorList tail: RFCOMM UUID16 then uint8 channel 8.
	buf := []byte{0x07, 0x00, 0x01, 0x00, 0x10, 0x35, 0x06, 0x19, 0x00, 0x03, 0x08, 0x08, 0x00}
	ch, ok := channelFromSDP(buf)
	if !ok || ch != 8 {
		t.Fatalf("ch=%d ok=%v", ch, ok)
	}
	if _, ok := channelFromSDP([]byte{0x19, 0x00, 0x03, 0x08, 0x00}); ok {
		t.Fatal("channel 0")
	}
}

func TestAwaitChannelRetriesMissingService(t *testing.T) {
	calls := 0
	ch, err := awaitChannel(context.Background(), time.Millisecond, func() (uint8, error) {
		calls++
		if calls < 3 {
			return 0, ErrNoChannel
		}
		return 8, nil
	})
	if err != nil || ch != 8 || calls != 3 {
		t.Fatalf("calls=%d ch=%d err=%v", calls, ch, err)
	}
}

func TestAwaitChannelStopsWhenNotConnected(t *testing.T) {
	calls := 0
	down := errors.New("wpp: sdp connect: host is down")
	_, err := awaitChannel(context.Background(), time.Second, func() (uint8, error) {
		calls++
		return 0, down
	})
	if calls != 1 || !errors.Is(err, down) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestAwaitChannelCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := awaitChannel(ctx, time.Hour, func() (uint8, error) {
		cancel()
		return 0, ErrNoChannel
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

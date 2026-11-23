package livevid

import (
	"testing"
	"time"
)

func TestEncodeAnnexB(t *testing.T) {
	r, err := New(Options{FPS: 10, Bitrate: 400, Encoder: "software"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	au, err := r.Encode(time.Date(2026, 8, 12, 0, 8, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(au) < 32 {
		t.Fatalf("AU too small: %d", len(au))
	}
	if !(au[0] == 0 && au[1] == 0 && au[2] == 0 && au[3] == 1) &&
		!(au[0] == 0 && au[1] == 0 && au[2] == 1) {
		t.Fatalf("not Annex-B: %x", au[:8])
	}

	// Second frame (P) should still be non-empty.
	au2, err := r.Encode(time.Date(2026, 8, 12, 0, 8, 1, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(au2) == 0 {
		t.Fatal("empty P-frame")
	}
	t.Logf("IDR/first=%dB P=%dB", len(au), len(au2))
	var max int
	for i := 0; i < 40; i++ {
		a, err := r.Encode(time.Date(2026, 8, 12, 0, 8, 0, 0, time.UTC).Add(time.Duration(i) * 100 * time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		if len(a) > max {
			max = len(a)
		}
	}
	t.Logf("max AU over 40 frames=%dB", max)
	if max > 12000 {
		t.Fatalf("AU too large for USB comfort: %dB", max)
	}
}

func TestForceIDRAfterPFrames(t *testing.T) {
	r, err := New(Options{FPS: 10, Bitrate: 400, Encoder: "software"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	base := time.Date(2026, 8, 12, 0, 8, 0, 0, time.UTC)
	first, err := r.Encode(base)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIDR(first) {
		t.Fatal("first AU should be IDR")
	}
	var p []byte
	for i := 1; i < 8; i++ {
		p, err = r.Encode(base.Add(time.Duration(i) * 100 * time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
	}
	if hasIDR(p) {
		t.Fatal("later AU should be P")
	}
	r.ForceIDR()
	idr, err := r.Encode(base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !hasIDR(idr) {
		t.Fatalf("ForceIDR AU not IDR (%dB)", len(idr))
	}
}

func TestFastEncoderOpens(t *testing.T) {
	r, err := New(Options{Width: 64, Height: 64, FPS: 10, Bitrate: 400, Encoder: "software-fast"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.EncoderName() != "x264-sw-fast" {
		t.Fatalf("name %q", r.EncoderName())
	}
	au, err := r.Encode(time.Date(2026, 8, 12, 0, 8, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(au) < 8 {
		t.Fatalf("AU %d", len(au))
	}
}

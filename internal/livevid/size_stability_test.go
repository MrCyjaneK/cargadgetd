package livevid_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/livevid"
)

func TestEncodeEveryFrame(t *testing.T) {
	r, err := livevid.New(livevid.Options{FPS: 10, Bitrate: 150, Encoder: "software"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	base := time.Date(2026, 8, 12, 15, 0, 0, 0, time.Local)
	idr, err := r.Encode(base)
	if err != nil {
		t.Fatal(err)
	}
	var maxP int
	var prev []byte
	for i := 1; i <= 25; i++ {
		au, err := r.Encode(base.Add(time.Duration(i) * 40 * time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		if prev != nil && bytes.Equal(au, prev) {
			t.Fatalf("encode-every produced identical AU at #%d (frame_num should advance)", i)
		}
		if len(au) > maxP {
			maxP = len(au)
		}
		if len(au) > 2000 {
			t.Fatalf("P#%d too large: %dB", i, len(au))
		}
		prev = append([]byte(nil), au...)
	}
	t.Logf("IDR=%dB maxP=%dB", len(idr), maxP)
}

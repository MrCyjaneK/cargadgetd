package session

import (
	"io"
	"log"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func TestLIVIDisplaySelection(t *testing.T) {
	cfg.SetForTest(t, nil)
	vc := []byte{0x08, 0x02} // 1280x720
	mainMedia := append([]byte{0x08, 0x03}, encodeLenKV(4, vc)...)
	clusterMedia := append(append([]byte{}, mainMedia...), encodeProtoVarint(6, 1)...)
	clusterMedia = append(clusterMedia, encodeProtoVarint(7, 1)...)
	mainVideo := append(encodeProtoVarint(1, 3), encodeLenKV(3, mainMedia)...)
	clusterVideo := append(encodeProtoVarint(1, 19), encodeLenKV(3, clusterMedia)...)

	touch := encodeLenKV(2, []byte{0x08, 0x80, 0x0a, 0x10, 0xd0, 0x05}) // 1280x720
	mainInput := append(encodeProtoVarint(1, 8), encodeLenKV(4, touch)...)
	clusterInput := append(encodeProtoVarint(1, 20), encodeLenKV(4, encodeProtoVarint(5, 1))...)

	// LIVI advertises main video, then cluster, then cluster input, then the touchscreen.
	assertDisplay(t, [][]byte{mainVideo, clusterVideo, clusterInput, mainInput})
	// Cluster last must not steal the main display or the touchscreen.
	assertDisplay(t, [][]byte{clusterVideo, mainVideo, mainInput, clusterInput})
}

func assertDisplay(t *testing.T, services [][]byte) {
	t.Helper()
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	var body []byte
	for _, svc := range services {
		body = append(body, encodeLenKV(1, svc)...)
	}
	m.parseSDR(body)
	if m.VideoCh != 3 {
		t.Fatalf("video channel %d, want main 3", m.VideoCh)
	}
	if m.InputCh != 8 {
		t.Fatalf("input channel %d, want touchscreen 8", m.InputCh)
	}
	if m.videoW != 1280 || m.videoH != 720 {
		t.Fatalf("video size %dx%d", m.videoW, m.videoH)
	}
	if m.inputW != 1280 || m.inputH != 720 {
		t.Fatalf("touch %dx%d", m.inputW, m.inputH)
	}
	got := map[uint8]string{}
	for _, s := range m.side {
		got[s.id] = s.kind
	}
	if got[19] != "video" || got[20] != "input" {
		t.Fatalf("side %v", got)
	}
}

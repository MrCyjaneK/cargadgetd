package session

import (
	"bytes"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func TestKnownMessagesLogWithoutChangingVideo(t *testing.T) {
	cfg.SetForTest(t, func(c *cfg.Config) { c.Apps.Strict = true })
	var buf bytes.Buffer
	m := NewMD(nil, nil, log.New(&buf, "", 0))
	m.videoStreaming.Store(true)

	// ByeByeRequest reason=POWER_DOWN (field 1 = 7).
	if err := m.handle(0, 0, []byte{0x00, 0x0f, 0x08, 0x07}); err != nil {
		t.Fatal(err)
	}
	// VideoFocusRequest mode=NATIVE (field 2 = 2), reason=LAUNCH_NATIVE (field 3 = 2).
	if err := m.handle(m.VideoCh, 0, []byte{0x80, 0x07, 0x10, 0x02, 0x18, 0x02}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("logged messages should leave video streaming")
	}
	if msg, ok := tryRecv(m); ok {
		t.Fatalf("logged message enqueued %x", msg.payload)
	}
	got := buf.String()
	if !strings.Contains(got, "byebye request reason=POWER_DOWN") {
		t.Fatalf("log %q", got)
	}
	if !strings.Contains(got, "video focus request mode=NATIVE reason=LAUNCH_NATIVE") {
		t.Fatalf("log %q", got)
	}

	if err := m.handle(0, 0, []byte{0x00, 0x20}); err == nil {
		t.Fatal("unknown control message should stay fatal in strict mode")
	}
}

func TestSideChannelsAreOpenedForLogging(t *testing.T) {
	cfg.SetForTest(t, func(c *cfg.Config) { c.Apps.Strict = true })
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	m.videoStreaming.Store(true)

	// id 4 PCM media sink (stream type 3), id 5 guidance (stream type 1).
	services := [][]byte{
		{0x08, 0x04, 0x1a, 0x04, 0x08, 0x01, 0x10, 0x03},
		{0x08, 0x05, 0x1a, 0x04, 0x08, 0x01, 0x10, 0x01},
		{0x08, 0x09, 0x32, 0x00}, // bluetooth
		{0x08, 0x0a, 0x42, 0x00}, // navigation
		{0x08, 0x0b, 0x3a, 0x00}, // radio
		{0x08, 0x0c, 0x52, 0x00}, // phone status
	}
	var body []byte
	for _, svc := range services {
		body = append(body, 0x0a, byte(len(svc)))
		body = append(body, svc...)
	}
	m.parseSDR(body)

	if m.AudioCh != 4 {
		t.Fatalf("audio channel %d, want the media sink", m.AudioCh)
	}
	got := map[uint8]string{}
	for _, s := range m.side {
		got[s.id] = s.kind
	}
	want := map[uint8]string{
		5:  "audio/guidance",
		9:  "bluetooth",
		10: "navigation",
		11: "radio",
		12: "phone_status",
	}
	if len(got) != len(want) {
		t.Fatalf("side %v", got)
	}
	for id, kind := range want {
		if got[id] != kind {
			t.Fatalf("channel %d kind %q, want %q", id, got[id], kind)
		}
	}

	if err := m.handle(9, 0, []byte{0x00, 0x2a, 0x08, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("a side-channel message should leave video streaming")
	}

	id, kind, ok := sideFromUpdate(append([]byte{0x0a, 0x04}, services[2]...))
	if !ok || id != 9 || kind != "bluetooth" {
		t.Fatalf("update id=%d kind=%s ok=%v", id, kind, ok)
	}
}

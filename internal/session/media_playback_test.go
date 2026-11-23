package session

import (
	"io"
	"log"
	"testing"
)

func TestPlaybackMetadataAndStatusFields(t *testing.T) {
	meta := EncodePlaybackMetadata("Night", "fakemink", "LP", []byte{1, 2, 3}, 180)
	title, ok := protoBytesField(meta, 1)
	if !ok || string(title) != "Night" {
		t.Fatalf("title %q %v", title, ok)
	}
	artist, ok := protoBytesField(meta, 2)
	if !ok || string(artist) != "fakemink" {
		t.Fatalf("artist %q", artist)
	}
	album, ok := protoBytesField(meta, 3)
	if !ok || string(album) != "LP" {
		t.Fatalf("album %q", album)
	}
	art, ok := protoBytesField(meta, 4)
	if !ok || string(art) != "\x01\x02\x03" {
		t.Fatalf("art %x", art)
	}
	dur, ok := protoVarintField(meta, 6)
	if !ok || dur != 180 {
		t.Fatalf("duration %d %v", dur, ok)
	}

	status := EncodePlaybackStatus(PlaybackPlaying, "Music", 12)
	state, ok := protoVarintField(status, 1)
	if !ok || state != PlaybackPlaying {
		t.Fatalf("state %d", state)
	}
	app, ok := protoBytesField(status, 2)
	if !ok || string(app) != "Music" {
		t.Fatalf("app %q", app)
	}
	pos, ok := protoVarintField(status, 3)
	if !ok || pos != 12 {
		t.Fatalf("position %d", pos)
	}
}

func TestClusterAndMainInputKeys(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	m.InputCh = 8
	m.side = []sideChannel{{id: 20, kind: "input"}, {id: 13, kind: "media_playback"}}
	// LIVI key report: timestamp 0, KeyEventBatch, next down (keycode 87).
	body := []byte{0x08, 0x00, 0x22, 0x0a, 0x0a, 0x08, 0x08, 0x57, 0x10, 0x01, 0x18, 0x00, 0x20, 0x00}
	full := append([]byte{0x80, 0x01}, body...)
	if err := m.handleChannel(20, 0, InputEvent, body, full); err != nil {
		t.Fatal(err)
	}
	ev := <-m.Events
	if ev.Type != "touch" || ev.Channel != 20 {
		t.Fatalf("cluster %+v", ev)
	}
	btns := ParseButtons(ev.Payload)
	if len(btns) != 1 || btns[0].ScanCode != 87 || !btns[0].Pressed {
		t.Fatalf("cluster next %v", btns)
	}
	name, ok := HuInputName(btns[0].ScanCode)
	if !ok || name != "next" {
		t.Fatalf("name %q", name)
	}

	if err := m.handleChannel(8, 0, InputEvent, body, full); err != nil {
		t.Fatal(err)
	}
	ev = <-m.Events
	if ev.Type != "touch" || ev.Channel != 8 {
		t.Fatalf("main %+v", ev)
	}
	btns = ParseButtons(ev.Payload)
	if len(btns) != 1 || btns[0].ScanCode != 87 || !btns[0].Pressed {
		t.Fatalf("main next %v", btns)
	}
}

func TestClusterPadAction(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	m.side = []sideChannel{{id: 13, kind: "media_playback"}}
	if err := m.handleChannel(13, 0, MediaPlaybackInput, []byte{0x08, 0x05}, []byte{0x80, 0x02, 0x08, 0x05}); err != nil {
		t.Fatal(err)
	}
	ev := <-m.Events
	if ev.Type != "huinput" || string(ev.Payload) != "dpad_click" {
		t.Fatalf("%+v", ev)
	}
	if err := m.handleChannel(13, 0, MediaPlaybackInput, []byte{0x08, 0x00}, []byte{0x80, 0x02, 0x08, 0x00}); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-m.Events:
		t.Fatalf("action 0 emitted %+v", ev)
	default:
	}
}

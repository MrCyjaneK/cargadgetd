package session

import "testing"

func TestParseButtonsField5(t *testing.T) {
	// ButtonEvent scan=87 next, pressed=1
	be := append(encodeProtoVarint(1, 87), encodeProtoVarint(2, 1)...)
	body := encodeLenKV(5, encodeLenKV(1, be))
	got := ParseButtons(body)
	if len(got) != 1 || got[0].ScanCode != 87 || !got[0].Pressed {
		t.Fatalf("%v", got)
	}
}

func TestParseButtonsField4Aasdk(t *testing.T) {
	be := append(encodeProtoVarint(1, 88), encodeProtoVarint(2, 0)...)
	msg := append([]byte{0x80, 0x01}, encodeLenKV(4, encodeLenKV(1, be))...)
	got := ParseButtons(msg)
	if len(got) != 1 || got[0].ScanCode != 88 || got[0].Pressed {
		t.Fatalf("%v", got)
	}
}

func TestParseButtonsSkipsTouch(t *testing.T) {
	loc := append(encodeProtoVarint(1, 400), encodeProtoVarint(2, 240)...)
	te := append(encodeLenKV(1, loc), encodeProtoVarint(3, 0)...)
	body := encodeLenKV(3, te)
	if got := ParseButtons(body); len(got) != 0 {
		t.Fatalf("touch parsed as buttons: %v", got)
	}
}

func TestParseLiviKeyReport(t *testing.T) {
	// LIVI InputReport: timestamp 0, KeyEventBatch, Home down.
	// native/crates/livi-aa-stack/src/channels/input.rs
	home := []byte{0x08, 0x00, 0x22, 0x0a, 0x0a, 0x08, 0x08, 0x03, 0x10, 0x01, 0x18, 0x00, 0x20, 0x00}
	got := ParseButtons(append([]byte{0x80, 0x01}, home...))
	if len(got) != 1 || got[0].ScanCode != 3 || !got[0].Pressed {
		t.Fatalf("home %v", got)
	}
	if n, ok := HuInputName(got[0].ScanCode); !ok || n != "home" {
		t.Fatalf("name %q %v", n, ok)
	}
	nextUp := []byte{0x08, 0x00, 0x22, 0x0a, 0x0a, 0x08, 0x08, 0x57, 0x10, 0x00, 0x18, 0x00, 0x20, 0x00}
	got = ParseButtons(nextUp)
	if len(got) != 1 || got[0].ScanCode != 87 || got[0].Pressed {
		t.Fatalf("next up %v", got)
	}
	play := []byte{0x08, 0x00, 0x22, 0x0a, 0x0a, 0x08, 0x08, 0x7e, 0x10, 0x01, 0x18, 0x00, 0x20, 0x00}
	got = ParseButtons(play)
	if len(got) != 1 || got[0].ScanCode != 126 || !got[0].Pressed {
		t.Fatalf("play %v", got)
	}
}

func TestParseLiviKnob(t *testing.T) {
	// LIVI rotary(0, -1): RelativeEventBatch, RotaryController, delta -1.
	rel := []byte{0x08, 0x80, 0x80, 0x04, 0x10}
	rel = append(rel, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}...)
	body := append([]byte{0x08, 0x00, 0x32, 0x11, 0x0a, 0x0f}, rel...)
	if got := ParseButtons(body); len(got) != 0 {
		t.Fatalf("knob parsed as a key: %v", got)
	}
	got := ParseKnobs(body)
	if len(got) != 1 || got[0].Code != 65536 || got[0].Delta != -1 {
		t.Fatalf("%v", got)
	}
	name, ok := KnobEvent(got[0].Code, got[0].Delta)
	if !ok || name != "rotary_ccw" {
		t.Fatalf("name %q %v", name, ok)
	}
	cw := []byte{0x08, 0x00, 0x32, 0x08, 0x0a, 0x06, 0x08, 0x80, 0x80, 0x04, 0x10, 0x02}
	got = ParseKnobs(cw)
	if len(got) != 2 || got[0].Delta != 1 || got[1].Delta != 1 {
		t.Fatalf("two detents %v", got)
	}
}

func TestParseButtonsKnownAndRelease(t *testing.T) {
	down := append(encodeProtoVarint(1, 85), encodeProtoVarint(2, 1)...)
	up := append(encodeProtoVarint(1, 85), encodeProtoVarint(2, 0)...)
	ev := append(encodeLenKV(1, down), encodeLenKV(1, up)...)
	got := ParseButtons(encodeLenKV(5, ev))
	if len(got) != 2 || !got[0].Pressed || got[1].Pressed {
		t.Fatalf("%v", got)
	}
	if n, ok := HuInputName(got[0].ScanCode); !ok || n != "playpause" {
		t.Fatalf("name %q %v", n, ok)
	}
}

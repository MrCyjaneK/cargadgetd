package session

// Button is a parsed InputEvent key (ButtonsEvent).
type Button struct {
	ScanCode int
	Pressed  bool
}

// ParseButtons extracts ButtonEvents from an InputEvent payload.
// Accepts ButtonsEvent as field 4 (aasdk) or field 5 (aa.proto). Field 3 is
// TouchEvent and is skipped. Inner field 2 > 1 is treated as a touch Y, not
// is_pressed, so TouchLocation blobs are not mistaken for keys.
func ParseButtons(msg []byte) []Button {
	body := msg
	if len(msg) >= 2 && msg[0] == 0x80 && msg[1] == 0x01 {
		body = msg[2:]
	}
	var out []Button
	i := 0
	for i < len(body) {
		tag, n := readVarint(body[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(body[i:])
			i += n2
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(body[i:])
		i += n2
		if i+int(l) > len(body) {
			break
		}
		sub := body[i : i+int(l)]
		i += int(l)
		if fn == 3 {
			continue
		}
		out = append(out, parseButtonsEvent(sub)...)
	}
	return out
}

func parseButtonsEvent(b []byte) []Button {
	var out []Button
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(b[i:])
			i += n2
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(b[i:])
		i += n2
		if i+int(l) > len(b) {
			break
		}
		sub := b[i : i+int(l)]
		i += int(l)
		if fn != 1 {
			continue
		}
		if btn, ok := parseButtonEvent(sub); ok {
			out = append(out, btn)
		}
	}
	return out
}

func parseButtonEvent(b []byte) (Button, bool) {
	code, ok := protoVarintField(b, 1)
	if !ok || code == 0 {
		return Button{}, false
	}
	pressed, hasPressed := protoVarintField(b, 2)
	if hasPressed && pressed > 1 {
		return Button{}, false
	}
	return Button{ScanCode: int(code), Pressed: !hasPressed || pressed != 0}, true
}

// Knob is one detent of a relative input (LIVI RelativeEvent).
type Knob struct {
	Code  int
	Delta int
}

// ParseKnobs reads RelativeEventBatch (InputReport field 6). A multi-detent
// delta becomes that many ±1 steps, capped so a corrupt value cannot flood
// the UI thread.
func ParseKnobs(msg []byte) []Knob {
	body := msg
	if len(msg) >= 2 && msg[0] == 0x80 && msg[1] == 0x01 {
		body = msg[2:]
	}
	var out []Knob
	for _, batch := range protoBytesFields(body, 6) {
		for _, ev := range protoBytesFields(batch, 1) {
			code, ok := protoVarintField(ev, 1)
			if !ok || code == 0 {
				continue
			}
			raw, ok := protoVarintField(ev, 2)
			if !ok {
				continue
			}
			delta := int(int32(raw))
			if delta == 0 {
				continue
			}
			n := delta
			if n < 0 {
				n = -n
			}
			if n > 8 {
				n = 8
			}
			step := 1
			if delta < 0 {
				step = -1
			}
			for i := 0; i < n; i++ {
				out = append(out, Knob{Code: int(code), Delta: step})
			}
		}
	}
	return out
}

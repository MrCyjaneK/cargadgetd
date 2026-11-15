package session

// Touch is a parsed InputEvent touch.
type Touch struct {
	X, Y   uint32
	Action int // 0=Press 1=Release 2=Drag 5=Down 6=Up
}

// ParseTouch extracts the first touch location from an InputEvent payload
// (message body after the 2-byte type, or full message including type).
func ParseTouch(msg []byte) (Touch, bool) {
	body := msg
	if len(msg) >= 2 {
		// Strip message type if present (0x8001).
		if msg[0] == 0x80 && msg[1] == 0x01 {
			body = msg[2:]
		}
	}
	// InputEvent field 3 (TouchEvent) wire type 2
	te, ok := protoBytesField(body, 3)
	if !ok {
		return Touch{}, false
	}
	action, _ := protoVarintField(te, 3)
	loc, ok := protoBytesField(te, 1)
	if !ok {
		return Touch{}, false
	}
	x, _ := protoVarintField(loc, 1)
	y, _ := protoVarintField(loc, 2)
	return Touch{X: uint32(x), Y: uint32(y), Action: int(action)}, true
}

func protoBytesFields(b []byte, field int) [][]byte {
	var out [][]byte
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
			if n2 == 0 {
				break
			}
			i += n2
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(b[i:])
		if n2 == 0 {
			break
		}
		i += n2
		if i+int(l) > len(b) {
			break
		}
		sub := b[i : i+int(l)]
		i += int(l)
		if fn == field {
			out = append(out, sub)
		}
	}
	return out
}

func protoBytesField(b []byte, field int) ([]byte, bool) {
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			return nil, false
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(b[i:])
			i += n2
			continue
		}
		if wt == 2 {
			l, n2 := readVarint(b[i:])
			i += n2
			if i+int(l) > len(b) {
				return nil, false
			}
			sub := b[i : i+int(l)]
			i += int(l)
			if fn == field {
				return sub, true
			}
			continue
		}
		return nil, false
	}
	return nil, false
}

func protoVarintField(b []byte, field int) (uint64, bool) {
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			return 0, false
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(b[i:])
			i += n2
			if fn == field {
				return v, true
			}
			continue
		}
		if wt == 2 {
			l, n2 := readVarint(b[i:])
			i += n2
			i += int(l)
			continue
		}
		return 0, false
	}
	return 0, false
}

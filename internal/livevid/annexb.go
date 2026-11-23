package livevid

// toAnnexB converts Android length-prefixed NALs or avcC into start-code
// Annex-B. codecConfig (SPS/PPS or avcC) is prepended on IDRs when missing.
func toAnnexB(au []byte, codecConfig []byte, sync bool) []byte {
	return toAnnexBSized(au, codecConfig, sync, 4)
}

func toAnnexBSized(au []byte, codecConfig []byte, sync bool, nalLen int) []byte {
	if len(au) == 0 {
		return au
	}
	nal := normalizeNAL(au, nalLen)
	// Some HALs omit BUFFER_FLAG_SYNC on the first IDR after a separate
	// codec-config buffer. Prepend SPS/PPS whenever the picture needs them.
	if len(codecConfig) > 0 && !hasSPS(nal) && (sync || hasIDR(nal)) {
		cfg := codecConfigToAnnexB(codecConfig)
		if len(cfg) > 0 {
			out := make([]byte, 0, len(cfg)+len(nal))
			out = append(out, cfg...)
			out = append(out, nal...)
			return out
		}
	}
	return nal
}

func normalizeNAL(in []byte, nalLen int) []byte {
	if isAnnexB(in) {
		return in
	}
	if cfg, nsz, consumed := parseAvcC(in); consumed > 0 && consumed <= len(in) {
		rest := in[consumed:]
		if len(rest) == 0 {
			return cfg
		}
		return append(cfg, lengthPrefixedToAnnexBSize(rest, nsz)...)
	}
	return lengthPrefixedToAnnexBSize(in, nalLen)
}

func codecConfigToAnnexB(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	if isAnnexB(in) {
		return in
	}
	if out, _, consumed := parseAvcC(in); consumed > 0 {
		return out
	}
	return lengthPrefixedToAnnexB(in)
}

func parseAvcC(in []byte) (out []byte, nalSize, consumed int) {
	if len(in) < 7 || in[0] != 1 {
		return nil, 4, 0
	}
	nalSize = int(in[4]&3) + 1
	if nalSize < 1 || nalSize > 4 {
		return nil, 4, 0
	}
	i := 5
	nSPS := int(in[i] & 0x1f)
	i++
	for s := 0; s < nSPS; s++ {
		if i+2 > len(in) {
			return nil, nalSize, 0
		}
		n := int(in[i])<<8 | int(in[i+1])
		i += 2
		if n <= 0 || i+n > len(in) {
			return nil, nalSize, 0
		}
		out = append(out, 0, 0, 0, 1)
		out = append(out, in[i:i+n]...)
		i += n
	}
	if i >= len(in) {
		return out, nalSize, i
	}
	nPPS := int(in[i])
	i++
	for p := 0; p < nPPS; p++ {
		if i+2 > len(in) {
			break
		}
		n := int(in[i])<<8 | int(in[i+1])
		i += 2
		if n <= 0 || i+n > len(in) {
			break
		}
		out = append(out, 0, 0, 0, 1)
		out = append(out, in[i:i+n]...)
		i += n
	}
	if len(out) == 0 {
		return nil, nalSize, 0
	}
	return out, nalSize, i
}

func isAnnexB(b []byte) bool {
	if len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 1 {
		return true
	}
	return len(b) >= 3 && b[0] == 0 && b[1] == 0 && b[2] == 1
}

func lengthPrefixedToAnnexB(in []byte) []byte {
	return lengthPrefixedToAnnexBSize(in, 4)
}

func lengthPrefixedToAnnexBSize(in []byte, nsize int) []byte {
	if nsize < 1 || nsize > 4 {
		nsize = 4
	}
	var out []byte
	i := 0
	for i+nsize <= len(in) {
		n := 0
		for b := 0; b < nsize; b++ {
			n = (n << 8) | int(in[i+b])
		}
		i += nsize
		if n <= 0 || i+n > len(in) {
			if len(out) == 0 {
				return in
			}
			return out
		}
		out = append(out, 0, 0, 0, 1)
		out = append(out, in[i:i+n]...)
		i += n
	}
	if len(out) == 0 {
		return in
	}
	return out
}

func hasSPS(annexB []byte) bool {
	return hasNALType(annexB, 7)
}

func hasIDR(annexB []byte) bool {
	return hasNALType(annexB, 5)
}

func hasNALType(annexB []byte, want byte) bool {
	ok := false
	walkNALs(annexB, func(t byte, _ []byte) bool {
		if t == want {
			ok = true
			return false
		}
		return true
	})
	return ok
}

// parameterSetsOnly reports SPS/PPS (and AUD/SEI/filler) with no VCL slice.
// Spreadtrum droidmedia emits this as the first encoder buffer without
// BUFFER_FLAG_CODEC_CONFIG - sending it as video AU #1 makes Subaru drop.
func parameterSetsOnly(in []byte) bool {
	if len(in) == 0 {
		return false
	}
	if cfg, _, consumed := parseAvcC(in); consumed > 0 && consumed == len(in) && hasSPS(cfg) {
		return true
	}
	sawPS := false
	onlyPS := true
	walkNALs(normalizeNAL(in, 4), func(t byte, _ []byte) bool {
		switch t {
		case 7, 8: // SPS, PPS
			sawPS = true
		case 6, 9, 12: // SEI, AUD, filler
		default:
			onlyPS = false
			return false
		}
		return true
	})
	return sawPS && onlyPS
}

func walkNALs(annexB []byte, fn func(nalType byte, nal []byte) bool) {
	i := 0
	for i < len(annexB) {
		var sc int
		if i+3 < len(annexB) && annexB[i] == 0 && annexB[i+1] == 0 && annexB[i+2] == 0 && annexB[i+3] == 1 {
			sc = 4
		} else if i+2 < len(annexB) && annexB[i] == 0 && annexB[i+1] == 0 && annexB[i+2] == 1 {
			sc = 3
		} else {
			i++
			continue
		}
		nal := i + sc
		if nal >= len(annexB) {
			return
		}
		j := nal + 1
		for j < len(annexB) {
			if j+4 <= len(annexB) && annexB[j] == 0 && annexB[j+1] == 0 && annexB[j+2] == 0 && annexB[j+3] == 1 {
				break
			}
			if j+3 <= len(annexB) && annexB[j] == 0 && annexB[j+1] == 0 && annexB[j+2] == 1 {
				break
			}
			j++
		}
		if !fn(annexB[nal]&0x1f, annexB[nal:j]) {
			return
		}
		i = j
	}
}

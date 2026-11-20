package session

import (
	"fmt"
	"strings"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

// AA VideoResolution enum (aasdk proto).
const (
	VideoRes800x480   = 1
	VideoRes1280x720  = 2
	VideoRes1920x1080 = 3
)

func videoSizeForEnum(e int) (w, h int) {
	switch e {
	case VideoRes800x480:
		return 800, 480
	case VideoRes1280x720:
		return 1280, 720
	case VideoRes1920x1080:
		return 1920, 1080
	default:
		return 0, 0
	}
}

// videoCfg is one HU VideoConfig: codec bucket + total (both-sides) margins.
type videoCfg struct {
	W, H    int
	MarginW int
	MarginH int
}

func (c videoCfg) pad() (ox, oy, cw, ch int) {
	return videoPad(c.W, c.H, c.MarginW, c.MarginH)
}

func videoPad(w, h, mw, mh int) (ox, oy, cw, ch int) {
	if w < 1 {
		w = 800
	}
	if h < 1 {
		h = 480
	}
	if mw < 0 {
		mw = 0
	}
	if mh < 0 {
		mh = 0
	}
	if mw >= w || mh >= h {
		return 0, 0, w, h
	}
	return mw / 2, mh / 2, w - mw, h - mh
}

// VideoSize is the HU codec geometry after SDR (default 800×480).
func (m *MD) VideoSize() (w, h int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.videoW <= 0 || m.videoH <= 0 {
		return 800, 480
	}
	return m.videoW, m.videoH
}

// VideoPad is the letterbox inset: UI lives in [ox,oy)+[cw,ch) of the codec frame.
// margin_width/height are totals split evenly (HUIG / GAL).
func (m *MD) VideoPad() (ox, oy, cw, ch int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return videoPad(m.videoW, m.videoH, m.videoMW, m.videoMH)
}

// TouchSize is the HU InputChannel touch-screen size (0 if missing).
func (m *MD) TouchSize() (w, h int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inputW, m.inputH
}

func parseVideoConfig(vc []byte) videoCfg {
	var c videoCfg
	i := 0
	for i < len(vc) {
		tag, n := readVarint(vc[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(vc[i:])
			i += n2
			switch fn {
			case 1:
				c.W, c.H = videoSizeForEnum(int(v))
			case 3:
				c.MarginW = int(v)
			case 4:
				c.MarginH = int(v)
			}
			continue
		}
		if wt == 2 {
			l, n2 := readVarint(vc[i:])
			i += n2
			i += int(l)
			continue
		}
		break
	}
	return c
}

func parseMediaVideoConfigs(media []byte) []videoCfg {
	var out []videoCfg
	i := 0
	for i < len(media) {
		tag, n := readVarint(media[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(media[i:])
			i += n2
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(media[i:])
		i += n2
		sub := media[i : i+int(l)]
		i += int(l)
		if fn != 4 {
			continue
		}
		c := parseVideoConfig(sub)
		if c.W > 0 && c.H > 0 {
			out = append(out, c)
		}
	}
	return out
}

func lengthField(b []byte, field int) []byte {
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			return nil
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(b[i:])
			i += n2
			continue
		}
		if wt != 2 {
			return nil
		}
		l, n2 := readVarint(b[i:])
		i += n2
		if i+int(l) > len(b) {
			return nil
		}
		sub := b[i : i+int(l)]
		i += int(l)
		if fn == field {
			return sub
		}
	}
	return nil
}

func parseTouchConfig(tc []byte) (w, h int) {
	i := 0
	for i < len(tc) {
		tag, n := readVarint(tc[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt != 0 {
			if wt == 2 {
				l, n2 := readVarint(tc[i:])
				i += n2
				i += int(l)
				continue
			}
			break
		}
		v, n2 := readVarint(tc[i:])
		i += n2
		switch fn {
		case 1:
			w = int(v)
		case 2:
			h = int(v)
		}
	}
	return w, h
}

func parseInputScreenSize(ch []byte) (w, h int) {
	// Channel.input_channel = 4 → InputChannel.screen_config = 2 → width/height.
	// Some HUs wrap TouchConfig in TouchScreen { config = 1 }.
	if in := lengthField(ch, 4); len(in) > 0 {
		if w, h = parseTouchConfigNested(lengthField(in, 2)); w > 0 && h > 0 {
			return w, h
		}
		if w, h = parseTouchConfigNested(in); w > 0 && h > 0 {
			return w, h
		}
	}
	if w, h = parseTouchConfigNested(lengthField(ch, 2)); w > 0 && h > 0 {
		return w, h
	}
	return parseTouchConfigNested(ch)
}

func parseTouchConfigNested(b []byte) (w, h int) {
	if w, h = parseTouchConfig(b); w > 0 && h > 0 {
		return w, h
	}
	return parseTouchConfig(lengthField(b, 1))
}

func parseInputButtons(ch []byte) []int {
	in := lengthField(ch, 4)
	if len(in) == 0 {
		in = ch
	}
	var out []int
	i := 0
	for i < len(in) {
		tag, n := readVarint(in[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(in[i:])
			i += n2
			if fn == 1 {
				out = append(out, int(v))
			}
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(in[i:])
		i += n2
		if i+int(l) > len(in) {
			break
		}
		sub := in[i : i+int(l)]
		i += int(l)
		// DHU packs available_buttons as a length-delimited blob of varints
		// (proto packed repeated), not as repeated field-1 varints.
		if fn == 1 {
			out = append(out, parsePackedVarints(sub)...)
		}
	}
	return out
}

func parsePackedVarints(b []byte) []int {
	var out []int
	i := 0
	for i < len(b) {
		v, n := readVarint(b[i:])
		if n == 0 {
			break
		}
		i += n
		out = append(out, int(v))
	}
	return out
}

// videoWant reads video.size (WxH or auto) and video.config_index.
// Unset / auto → (-1 or configured index, 0, 0).
func videoWant() (idx, w, h int) {
	v := cfg.Get().Video
	idx = v.ConfigIndex
	if idx < 0 {
		idx = -1
	}
	s := strings.TrimSpace(v.Size)
	if s == "" || strings.EqualFold(s, "auto") {
		return idx, 0, 0
	}
	var ww, hh int
	if _, err := fmt.Sscanf(s, "%dx%d", &ww, &hh); err == nil && ww > 0 && hh > 0 {
		return idx, ww, hh
	}
	return idx, 0, 0
}

// pickVideoConfig prefers an exact codec WxH. Content-rect matches are a
// fallback and take the smallest codec (Subaru lists 1080p-with-800x480-UI
// first; matching the inner UI used to select 1080p).
func pickVideoConfig(cfgs []videoCfg, w, h int) (videoCfg, int, bool) {
	if w < 1 || h < 1 {
		return videoCfg{}, 0, false
	}
	for i, c := range cfgs {
		if c.W == w && c.H == h {
			return c, i, true
		}
	}
	best := -1
	for i, c := range cfgs {
		_, _, cw, ch := c.pad()
		if cw != w || ch != h {
			continue
		}
		if best < 0 || c.W*c.H < cfgs[best].W*cfgs[best].H {
			best = i
		}
	}
	if best >= 0 {
		return cfgs[best], best, true
	}
	return videoCfg{}, 0, false
}

// selectVideoConfig picks the HU video_configs[] entry we will encode and
// declare in StartIndication (config_index).
//
// Order: explicit WxH (exact codec, else smallest content match) → explicit
// index → touch-screen match (same rule) → index 0.
func selectVideoConfig(cfgs []videoCfg, inputW, inputH, wantIdx, wantW, wantH int) (videoCfg, int) {
	if wantW > 0 && wantH > 0 {
		if c, i, ok := pickVideoConfig(cfgs, wantW, wantH); ok {
			return c, i
		}
		if len(cfgs) == 0 {
			return videoCfg{W: wantW, H: wantH}, 0
		}
	}
	if wantIdx >= 0 && wantIdx < len(cfgs) {
		return cfgs[wantIdx], wantIdx
	}
	if inputW > 0 && inputH > 0 {
		if c, i, ok := pickVideoConfig(cfgs, inputW, inputH); ok {
			return c, i
		}
		if len(cfgs) == 0 {
			return videoCfg{W: inputW, H: inputH}, 0
		}
	}
	if len(cfgs) > 0 {
		return cfgs[0], 0
	}
	return videoCfg{W: 800, H: 480}, 0
}

func formatVideoCfgs(cfgs []videoCfg) string {
	if len(cfgs) == 0 {
		return "(none)"
	}
	parts := make([]string, len(cfgs))
	for i, c := range cfgs {
		if c.MarginW != 0 || c.MarginH != 0 {
			_, _, cw, ch := c.pad()
			parts[i] = fmt.Sprintf("%d:%dx%d m=%dx%d ui=%dx%d", i, c.W, c.H, c.MarginW, c.MarginH, cw, ch)
		} else {
			parts[i] = fmt.Sprintf("%d:%dx%d", i, c.W, c.H)
		}
	}
	return strings.Join(parts, ",")
}

// MapTouch converts an HU touch into content-rectangle pixels.
// Codec-space coords are shifted by the letterbox origin; touch-screen size
// matching the content rectangle is used as-is; other sizes are scaled.
func MapTouch(x, y, ox, oy, cw, ch, codecW, codecH, inW, inH int) (int, int) {
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	// Some head units omit the touchscreen size from the input SDR
	// (touch=unset). Their InputEvent coordinates are then relative to the
	// projected content area, not to the full video frame. Treat them as UI
	// pixels: subtracting the video margins shifted Y into the top letterbox
	// and clamped every touch to y=0.
	if inW <= 0 || inH <= 0 {
		inW, inH = cw, ch
	}
	switch {
	case inW == cw && inH == ch:
		// already UI pixels
	case inW == codecW && inH == codecH:
		x -= ox
		y -= oy
	default:
		x = x * cw / inW
		y = y * ch / inH
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= cw {
		x = cw - 1
	}
	if y >= ch {
		y = ch - 1
	}
	return x, y
}

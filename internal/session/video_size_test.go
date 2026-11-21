package session

import (
	"encoding/hex"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func TestParseVideoConfig(t *testing.T) {
	// resolution=2 (720p), fps=1
	c := parseVideoConfig([]byte{0x08, 2, 0x10, 1})
	if c.W != 1280 || c.H != 720 {
		t.Fatalf("got %dx%d want 1280x720", c.W, c.H)
	}
	c = parseVideoConfig([]byte{0x08, 1})
	if c.W != 800 || c.H != 480 {
		t.Fatalf("480p: %dx%d", c.W, c.H)
	}
	c = parseVideoConfig([]byte{0x08, 3})
	if c.W != 1920 || c.H != 1080 {
		t.Fatalf("1080p: %dx%d", c.W, c.H)
	}
}

func TestParseVideoConfigMargins(t *testing.T) {
	// 720p, fps=30, margin_width=256, margin_height=120
	vc := []byte{0x08, 2, 0x10, 1, 0x18, 0x80, 0x02, 0x20, 0x78}
	c := parseVideoConfig(vc)
	if c.W != 1280 || c.H != 720 || c.MarginW != 256 || c.MarginH != 120 {
		t.Fatalf("got %+v", c)
	}
	ox, oy, cw, ch := c.pad()
	if ox != 128 || oy != 60 || cw != 1024 || ch != 600 {
		t.Fatalf("pad %d,%d %dx%d", ox, oy, cw, ch)
	}
}

func TestParseMediaVideoConfigsOrder(t *testing.T) {
	vc480 := []byte{0x08, 1}
	vc1080 := []byte{0x08, 3}
	mc := []byte{0x08, 3} // media_type video
	mc = append(mc, encodeLenKV(4, vc480)...)
	mc = append(mc, encodeLenKV(4, vc1080)...)
	got := parseMediaVideoConfigs(mc)
	if len(got) != 2 || got[0].W != 800 || got[1].W != 1920 {
		t.Fatalf("got %+v", got)
	}
}

func TestSelectVideoConfig(t *testing.T) {
	all := []videoCfg{
		{W: 800, H: 480},
		{W: 1280, H: 720, MarginW: 256, MarginH: 120}, // content 1024×600
		{W: 1920, H: 1080},
	}
	subaru := []videoCfg{
		{W: 1920, H: 1080, MarginW: 1120, MarginH: 600}, // ui 800×480
		{W: 1280, H: 720, MarginW: 480, MarginH: 240},   // ui 800×480
		{W: 800, H: 480},
	}
	cases := []struct {
		name                  string
		cfgs                  []videoCfg
		inW, inH, idx, ww, hh int
		wantW, wantH, wantI   int
	}{
		{name: "first when no hint", cfgs: all, idx: -1, wantW: 800, wantH: 480, wantI: 0},
		{name: "touch matches 720p codec", cfgs: all, inW: 1280, inH: 720, idx: -1, wantW: 1280, wantH: 720, wantI: 1},
		{name: "touch matches content", cfgs: all, inW: 1024, inH: 600, idx: -1, wantW: 1280, wantH: 720, wantI: 1},
		{name: "explicit size 480p", cfgs: all, inW: 1280, inH: 720, idx: -1, ww: 800, hh: 480, wantW: 800, wantH: 480, wantI: 0},
		{name: "explicit index 2", cfgs: all, idx: 2, wantW: 1920, wantH: 1080, wantI: 2},
		{name: "size wins over index", cfgs: all, idx: 2, ww: 800, hh: 480, wantW: 800, wantH: 480, wantI: 0},
		{name: "empty + touch", inW: 800, inH: 480, idx: -1, wantW: 800, wantH: 480, wantI: 0},
		{name: "empty default", idx: -1, wantW: 800, wantH: 480, wantI: 0},
		{name: "size not advertised", cfgs: all, idx: -1, ww: 640, hh: 480, wantW: 800, wantH: 480, wantI: 0},
		// Harman Subaru_Mid: 1080p/720p letterbox to 800×480, plus a native 480p bucket.
		{name: "subaru size 800x480 is codec not 1080p UI", cfgs: subaru, inW: 800, inH: 480, idx: -1, ww: 800, hh: 480, wantW: 800, wantH: 480, wantI: 2},
		{name: "subaru auto touch prefers native 480p", cfgs: subaru, inW: 800, inH: 480, idx: -1, wantW: 800, wantH: 480, wantI: 2},
		{name: "subaru content-only picks smallest", cfgs: subaru[:2], inW: 800, inH: 480, idx: -1, wantW: 1280, wantH: 720, wantI: 1},
	}
	for _, tc := range cases {
		c, i := selectVideoConfig(tc.cfgs, tc.inW, tc.inH, tc.idx, tc.ww, tc.hh)
		if c.W != tc.wantW || c.H != tc.wantH || i != tc.wantI {
			t.Errorf("%s: got %dx%d idx=%d want %dx%d idx=%d", tc.name, c.W, c.H, i, tc.wantW, tc.wantH, tc.wantI)
		}
	}
}

func TestMapTouch(t *testing.T) {
	// 1280x720 codec, margins 256x120 → content 1024x600 at origin 128,60
	ox, oy, cw, ch := 128, 60, 1024, 600
	codecW, codecH := 1280, 720
	x, y := MapTouch(128, 60, ox, oy, cw, ch, codecW, codecH, codecW, codecH)
	if x != 0 || y != 0 {
		t.Fatalf("codec-space origin: %d,%d", x, y)
	}
	x, y = MapTouch(0, 0, ox, oy, cw, ch, codecW, codecH, cw, ch)
	if x != 0 || y != 0 {
		t.Fatalf("content-space: %d,%d", x, y)
	}
	x, y = MapTouch(512, 300, ox, oy, cw, ch, codecW, codecH, 1024, 600)
	if x != 512 || y != 300 {
		t.Fatalf("scaled identity: %d,%d", x, y)
	}
	// HU that does not advertise a touchscreen size (touch=unset): the input
	// event is in content space. Regression: the margins used to be subtracted
	// and every Y clamped to 0.
	x, y = MapTouch(398, 114, ox, oy, cw, ch, codecW, codecH, 0, 0)
	if x != 398 || y != 114 {
		t.Fatalf("unset input size: %d,%d", x, y)
	}
}

func TestVideoWant(t *testing.T) {
	cfg.SetForTest(t, nil)
	idx, w, h := videoWant()
	if idx != -1 || w != 0 || h != 0 {
		t.Fatalf("unset: idx=%d %dx%d", idx, w, h)
	}
	cfg.SetForTest(t, func(c *cfg.Config) {
		c.Video.Size = "auto"
	})
	idx, w, h = videoWant()
	if idx != -1 || w != 0 {
		t.Fatalf("auto: idx=%d %dx%d", idx, w, h)
	}
	cfg.SetForTest(t, func(c *cfg.Config) {
		c.Video.Size = "800x480"
		c.Video.ConfigIndex = 2
	})
	idx, w, h = videoWant()
	if idx != 2 || w != 800 || h != 480 {
		t.Fatalf("both: idx=%d %dx%d", idx, w, h)
	}
}

func TestVideoPadIgnoresOversize(t *testing.T) {
	ox, oy, cw, ch := videoPad(800, 480, 800, 0)
	if ox != 0 || oy != 0 || cw != 800 || ch != 480 {
		t.Fatalf("%d,%d %dx%d", ox, oy, cw, ch)
	}
}

func TestParseInputScreenSizeChannelWrapped(t *testing.T) {
	// 1280x720 TouchConfig inside InputChannel (field 2) inside Channel (field 4).
	tc := []byte{0x08, 0x80, 0x0a, 0x10, 0xd0, 0x05}
	ic := encodeLenKV(2, tc)
	ch := encodeLenKV(4, ic)
	w, h := parseInputScreenSize(ch)
	if w != 1280 || h != 720 {
		t.Fatalf("wrapped: %dx%d body=%x", w, h, ch)
	}
	w, h = parseInputScreenSize(ic)
	if w != 1280 || h != 720 {
		t.Fatalf("bare input: %dx%d", w, h)
	}
}

func TestParseInputScreenSizeTouchScreenNested(t *testing.T) {
	tc := []byte{0x08, 0x80, 0x0a, 0x10, 0xd0, 0x05}
	ts := encodeLenKV(1, tc)
	ic := encodeLenKV(2, ts)
	ch := encodeLenKV(4, ic)
	w, h := parseInputScreenSize(ch)
	if w != 1280 || h != 720 {
		t.Fatalf("nested: %dx%d", w, h)
	}
}

func TestParseInputButtons(t *testing.T) {
	tc := []byte{0x08, 0x80, 0x0a, 0x10, 0xd0, 0x05}
	ic := []byte{0x08, 0x01, 0x08, 0x04, 0x08, 0x03}
	ic = append(ic, encodeLenKV(2, tc)...)
	ch := encodeLenKV(4, ic)
	got := parseInputButtons(ch)
	if len(got) != 3 || got[0] != 1 || got[1] != 4 || got[2] != 3 {
		t.Fatalf("%v", got)
	}
}

func TestParseInputButtonsPackedDHU(t *testing.T) {
	// DHU 2.0-linux SDR InputChannel (GAL_CAPTURE 2026-08-16): field 1 is
	// packed varints, not repeated 08 <code>.
	packed, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161751545557587e7f8c028d028e028f02808004818004828004848004")
	if err != nil {
		t.Fatal(err)
	}
	tc := []byte{0x08, 0xa0, 0x06, 0x10, 0xe0, 0x03, 0x18, 0x02, 0x20, 0x00, 0x28, 0x00}
	ic := append(encodeLenKV(1, packed), encodeLenKV(2, tc)...)
	ch := encodeLenKV(4, ic)
	got := parseInputButtons(ch)
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 81, 84, 85, 87, 88, 126, 127, 268, 269, 270, 271, 65536, 65537, 65538, 65540}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v want %v", got, want)
		}
	}
}

func encodeLenKV(field int, v []byte) []byte {
	tag := byte(field<<3 | 2)
	out := []byte{tag, byte(len(v))}
	return append(out, v...)
}

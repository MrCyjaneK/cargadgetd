package livevid

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"testing"
	"time"
)

type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Sleep(d time.Duration) {
	if d > 0 {
		f.t = f.t.Add(d)
	}
}

type fakeEnc struct {
	clk   *fakeClock
	delay func(n int) time.Duration
	n     int
}

func (f *fakeEnc) Name() string { return "fake" }

func (f *fakeEnc) Encode(*image.RGBA) ([]byte, error) {
	d := time.Millisecond
	if f.delay != nil {
		d = f.delay(f.n)
	}
	f.n++
	if d > 0 {
		f.clk.Sleep(d)
	}
	return []byte{0, 0, 0, 1}, nil
}

func (f *fakeEnc) Close() error { return nil }

func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.05 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCPUPercent(t *testing.T) {
	near(t, cpuPercent(0, 0, 900, 1000), 10)
	near(t, cpuPercent(900, 1000, 1000, 2000), 90)
	if cpuPercent(1, 1, 1, 1) != 0 {
		t.Fatal("zero delta")
	}
	idle, total, err := parseCPULine("cpu  10 20 30 40 5 1 0 0 0 0")
	if err != nil {
		t.Fatal(err)
	}
	if idle != 40 || total != 106 {
		t.Fatalf("idle %d total %d", idle, total)
	}
	if _, _, err := parseCPULine("cpu0 1 2 3 4"); err == nil {
		t.Fatal("per-cpu line accepted")
	}
}

func TestStripMoves(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	strip, h := prepareFrame(img)
	before := append([]byte(nil), img.Pix...)
	placeStrip(img, strip, h, 0)
	if bytes.Equal(before, img.Pix) {
		t.Fatal("strip did not change the frame")
	}
	first := append([]byte(nil), img.Pix...)
	placeStrip(img, strip, h, 4)
	if bytes.Equal(first, img.Pix) {
		t.Fatal("strip did not move")
	}
}

func TestBenchHeldAndOneLateFrame(t *testing.T) {
	clk := &fakeClock{}
	var stats [][2]uint64
	for c := 0; c < 3; c++ {
		base := uint64(c) * 10000
		stats = append(stats,
			[2]uint64{base + 900, base + 1000},
			[2]uint64{base + 1000, base + 2000},
		)
	}
	n := 0
	var status []string
	var opens []int
	res := Run(BenchOptions{
		Encoder: "software",
		Sizes:   [][2]int{{16, 16}},
		Clock:   clk,
		Stat: func() (uint64, uint64, error) {
			if n >= len(stats) {
				t.Fatalf("stat read %d", n)
			}
			s := stats[n]
			n++
			return s[0], s[1], nil
		},
		Open: func(w, h, fps, bps int, want string) (codec, error) {
			if want != "software" {
				t.Fatalf("encoder %q", want)
			}
			if w != 16 || h != 16 {
				t.Fatalf("size %dx%d", w, h)
			}
			if bps != hwBitrate(w, h, fps, 400) {
				t.Fatalf("bps %d", bps)
			}
			opens = append(opens, fps)
			enc := &fakeEnc{clk: clk}
			if fps == 30 {
				enc.delay = func(i int) time.Duration {
					if i == 0 {
						return 40 * time.Millisecond
					}
					return time.Millisecond
				}
			}
			return enc, nil
		},
		OnStatus: func(s string) { status = append(status, s) },
	})
	if len(res.Cells) != 3 {
		t.Fatalf("cells %d", len(res.Cells))
	}
	paced30 := res.Cells[0]
	if paced30.Mode != "30" || paced30.Held || !paced30.Late || paced30.Error != "" {
		t.Fatalf("30fps %+v", paced30)
	}
	near(t, paced30.Load, 90)
	paced60 := res.Cells[1]
	if !paced60.Held || paced60.Late || paced60.Error != "" {
		t.Fatalf("60fps %+v", paced60)
	}
	near(t, paced60.FPS, 60)
	near(t, paced60.Load, 90)
	if res.Cells[2].Mode != "max" || res.Cells[2].Held || res.Cells[2].Late {
		t.Fatalf("max %+v", res.Cells[2])
	}
	if res.Cells[2].FPS < 100 {
		t.Fatalf("max fps %v", res.Cells[2].FPS)
	}
	if !CanSelect(res.Cells) {
		t.Fatal("60fps held, encoder should be selectable")
	}
	if opens[0] != 30 || opens[1] != 60 || opens[2] != 60 {
		t.Fatalf("opens %v", opens)
	}
	wantStatus := []string{
		"Testing - 16×16 - 30 fps",
		"Testing - 16×16 - 60 fps",
		"Testing - 16×16 - max",
	}
	if len(status) != len(wantStatus) {
		t.Fatalf("status %q", status)
	}
	for i := range wantStatus {
		if status[i] != wantStatus[i] {
			t.Fatalf("status[%d]=%q", i, status[i])
		}
	}
}

func TestBenchStallStopsAndContinues(t *testing.T) {
	clk := &fakeClock{}
	res := Run(BenchOptions{
		Encoder: "droidmedia",
		Sizes:   [][2]int{{16, 16}},
		Clock:   clk,
		Stat:    func() (uint64, uint64, error) { return 0, 1, nil },
		Open: func(w, h, fps, bps int, want string) (codec, error) {
			enc := &fakeEnc{clk: clk}
			if fps == 30 {
				enc.delay = func(int) time.Duration { return 300 * time.Millisecond }
			}
			return enc, nil
		},
	})
	if len(res.Cells) != 3 {
		t.Fatalf("cells %d", len(res.Cells))
	}
	stalled := res.Cells[0]
	if stalled.Held || !stalled.Late || stalled.Reason != "encode took 300ms" {
		t.Fatalf("stall %+v", stalled)
	}
	if stalled.FPS > 10 {
		t.Fatalf("stall fps %v", stalled.FPS)
	}
	if !res.Cells[1].Held {
		t.Fatalf("next cell did not run: %+v", res.Cells[1])
	}
	if !CanSelect(res.Cells) {
		t.Fatal("later held cell should allow select")
	}
}

func TestBenchOpenErrorContinues(t *testing.T) {
	clk := &fakeClock{}
	res := Run(BenchOptions{
		Encoder: "droidmedia",
		Sizes:   [][2]int{{16, 16}},
		Clock:   clk,
		Stat:    func() (uint64, uint64, error) { return 1, 2, nil },
		Open: func(w, h, fps, bps int, want string) (codec, error) {
			if fps == 30 {
				return nil, fmt.Errorf("no video/avc encoder")
			}
			return &fakeEnc{clk: clk}, nil
		},
	})
	if res.Cells[0].Error != "no video/avc encoder" || res.Cells[0].Held || res.Cells[0].FPS != 0 {
		t.Fatalf("open %+v", res.Cells[0])
	}
	if !res.Cells[1].Held {
		t.Fatalf("next cell %+v", res.Cells[1])
	}
	if !CanSelect(res.Cells) {
		t.Fatal("held cell after an open error")
	}
}

func TestBenchAllErrorsCannotSelect(t *testing.T) {
	clk := &fakeClock{}
	var got []string
	res := Run(BenchOptions{
		Encoder: "software",
		Clock:   clk,
		Window:  time.Millisecond,
		Stat:    func() (uint64, uint64, error) { return 0, 1, nil },
		Open: func(w, h, fps, bps int, want string) (codec, error) {
			if fps == 30 {
				got = append(got, fmt.Sprintf("%dx%d", w, h))
			}
			return nil, fmt.Errorf("create_encoder failed")
		},
	})
	want := []string{"800x480", "1280x720", "1920x1080", "2560x1440", "3840x2160"}
	if len(got) != len(want) {
		t.Fatalf("sizes %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("size %s", got[i])
		}
	}
	if len(res.Cells) != len(BenchSizes)*3 {
		t.Fatalf("cells %d", len(res.Cells))
	}
	for _, c := range res.Cells {
		if c.Error != "create_encoder failed" || c.Held {
			t.Fatalf("cell %+v", c)
		}
	}
	if CanSelect(res.Cells) {
		t.Fatal("all errors must not select")
	}
}

func TestBenchReportsLiveFPS(t *testing.T) {
	clk := &fakeClock{}
	var saw []Cell
	Run(BenchOptions{
		Encoder: "software",
		Sizes:   [][2]int{{16, 16}},
		Clock:   clk,
		Stat:    func() (uint64, uint64, error) { return 0, 1, nil },
		Open: func(w, h, fps, bps int, want string) (codec, error) {
			return &fakeEnc{clk: clk}, nil
		},
		OnLive: func(c Cell) {
			if c.Mode == "30" {
				saw = append(saw, c)
			}
		},
	})
	if len(saw) < 2 {
		t.Fatalf("live samples %d", len(saw))
	}
	if saw[0].FPS <= 0 || saw[0].Elapsed <= 0 {
		t.Fatalf("first sample %+v", saw[0])
	}
	if saw[len(saw)-1].Elapsed <= saw[0].Elapsed {
		t.Fatalf("elapsed did not advance: %v then %v", saw[0].Elapsed, saw[len(saw)-1].Elapsed)
	}
	near(t, saw[len(saw)-1].FPS, 30)
}

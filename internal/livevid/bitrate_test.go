package livevid

import (
	"image"
	"testing"
	"time"
)

type fixedAU struct{ n int }

func (f *fixedAU) Name() string { return "fixed" }
func (f *fixedAU) Close() error { return nil }
func (f *fixedAU) Encode(*image.RGBA) ([]byte, error) {
	n := f.n
	if n < 5 {
		n = 5
	}
	b := make([]byte, n)
	b[0], b[1], b[2], b[3] = 0, 0, 0, 1
	return b, nil
}

func withCodecHook(t *testing.T, hook func(w, h, fps, bps int, want string) (codec, error)) {
	t.Helper()
	prev := openCodecHook
	openCodecHook = hook
	t.Cleanup(func() { openCodecHook = prev })
}

func TestFatAUSentWithoutReopen(t *testing.T) {
	opens := 0
	withCodecHook(t, func(_, _, _, _ int, _ string) (codec, error) {
		opens++
		return &fixedAU{n: 230_000}, nil
	})
	r := newHookRenderer(t)
	start := r.bps
	for i := 0; i < 3; i++ {
		au, err := r.Encode(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(au) != 230_000 {
			t.Fatalf("au %d", len(au))
		}
	}
	if r.bps != start {
		t.Fatalf("bps %d, want %d", r.bps, start)
	}
	if opens != 1 {
		t.Fatalf("opens %d, a fat frame must not recreate the encoder", opens)
	}
}

func TestPipeSlowStopsAtCrispFloor(t *testing.T) {
	withCodecHook(t, func(_, _, _, _ int, _ string) (codec, error) {
		return &fixedAU{n: 1000}, nil
	})
	r := newHookRenderer(t)
	start := r.bps
	if start <= crispFloor {
		t.Fatalf("start %d", start)
	}
	r.NotePipeSlow()
	if r.bps != start {
		t.Fatal("one slow sample must not cut")
	}
	r.NotePipeSlow()
	if r.bps >= start || r.bps < crispFloor {
		t.Fatalf("bps %d", r.bps)
	}
	held := r.bps
	r.NotePipeSlow()
	r.NotePipeSlow()
	if r.bps != held {
		t.Fatalf("cooldown bps %d, want %d", r.bps, held)
	}
	r.mu.Lock()
	r.bps = crispFloor
	r.lastChange = time.Now().Add(-time.Minute)
	r.mu.Unlock()
	r.NotePipeSlow()
	r.NotePipeSlow()
	if r.bps != crispFloor {
		t.Fatalf("floor %d", r.bps)
	}
}

func TestClimbAfterQuietFrames(t *testing.T) {
	prev := climbAfter
	climbAfter = 3
	t.Cleanup(func() { climbAfter = prev })
	opens := 0
	withCodecHook(t, func(_, _, _, _ int, _ string) (codec, error) {
		opens++
		return &fixedAU{n: 1000}, nil
	})
	r := newHookRenderer(t)
	r.mu.Lock()
	r.bps = crispFloor
	r.lastChange = time.Now().Add(-time.Minute)
	r.mu.Unlock()
	for i := 0; i < 3; i++ {
		if _, err := r.Encode(time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if r.bps != 2_000_000 {
		t.Fatalf("bps %d", r.bps)
	}
	if opens != 1 {
		t.Fatalf("climb must wait for the next frame, opens %d", opens)
	}
	if _, err := r.Encode(time.Now()); err != nil {
		t.Fatal(err)
	}
	if opens != 2 {
		t.Fatalf("opens %d", opens)
	}
}

func TestPipeSlowAfterClimbPinsCeiling(t *testing.T) {
	withCodecHook(t, func(_, _, _, _ int, _ string) (codec, error) {
		return &fixedAU{n: 1000}, nil
	})
	r := newHookRenderer(t)
	r.mu.Lock()
	r.climbedAt = time.Now()
	r.mu.Unlock()
	r.NotePipeSlow()
	r.NotePipeSlow()
	if r.bps != 3_600_000 || r.ceiling != 3_600_000 {
		t.Fatalf("bps %d ceiling %d", r.bps, r.ceiling)
	}
}

func newHookRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := New(Options{
		Width: 800, Height: 480, FPS: 30, Bitrate: 400,
		Encoder: "software",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

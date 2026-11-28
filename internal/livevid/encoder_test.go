package livevid

import "testing"

func TestHwBitrateUSBBudget(t *testing.T) {
	b := hwBitrate(1920, 1080, 30, 400)
	if b != 4_800_000 {
		t.Fatalf("1080p30: %d", b)
	}
	b = hwBitrate(800, 480, 30, 400)
	if b != 4_800_000 {
		t.Fatalf("480p30: %d", b)
	}
	b = hwBitrate(1280, 720, 10, 400)
	if b != 1_600_000 {
		t.Fatalf("720p10: %d", b)
	}
	if hwBitrate(800, 480, 30, 8000) != 8_000_000 {
		t.Fatal("hint should raise")
	}
	if hwBitrate(800, 480, 30, 2000) != 4_800_000 {
		t.Fatal("hint must not lower the crisp target")
	}
}

func TestStepBitrate(t *testing.T) {
	if stepDown(4_800_000, minBps) != 3_600_000 {
		t.Fatalf("quarter: %d", stepDown(4_800_000, minBps))
	}
	if stepDown(1_600_000, crispFloor) != crispFloor {
		t.Fatalf("pipe floor: %d", stepDown(1_600_000, crispFloor))
	}
	if stepDown(crispFloor, crispFloor) != crispFloor {
		t.Fatal("pipe step should stick at the crisp floor")
	}
	if stepDown(300_000, minBps) != minBps {
		t.Fatalf("min: %d", stepDown(300_000, minBps))
	}
	if stepUp(crispFloor, 4_800_000) != 2_000_000 {
		t.Fatalf("up: %d", stepUp(crispFloor, 4_800_000))
	}
	if stepUp(4_000_000, 4_800_000) != 4_800_000 {
		t.Fatalf("cap: %d", stepUp(4_000_000, 4_800_000))
	}
}

// An explicit target is what lets a session open below the frame budget. The
// budget stays the default; a head unit that reboots its decoder under a heavy
// stream needs the lower rate from the very first frame, not after a step-down
// that only happens once the unit has stopped acking.
func TestOpenRateWithTarget(t *testing.T) {
	cases := []struct {
		name string
		o    Options
		want int
	}{
		{"budget by default", Options{Width: 1280, Height: 720, FPS: 30}, 4_800_000},
		{"explicit 1500 kbps", Options{Width: 1280, Height: 720, FPS: 30, TargetBps: 1500 * 1000}, 1_500_000},
		{"below the floor is raised", Options{Width: 1280, Height: 720, FPS: 30, TargetBps: 100 * 1000}, minBps},
		{"above the ceiling is clamped", Options{Width: 1280, Height: 720, FPS: 30, TargetBps: 20_000_000}, 6_000_000},
	}
	for _, c := range cases {
		if got := openRate(c.o); got != c.want {
			t.Errorf("%s: openRate = %d, want %d", c.name, got, c.want)
		}
	}
}

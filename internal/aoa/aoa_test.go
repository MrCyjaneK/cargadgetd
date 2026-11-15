package aoa

import (
	"testing"
	"time"
)

func TestIsAndroidAuto(t *testing.T) {
	if !IsAndroidAuto("Android Open Automotive Protocol") {
		t.Fatal("DHU model")
	}
	if !IsAndroidAuto("Android Auto") {
		t.Fatal("car model")
	}
	if IsAndroidAuto("Pixel 9") || IsAndroidAuto("") {
		t.Fatal("unrelated model matched")
	}
}

func TestSampleGap(t *testing.T) {
	if sampleGapAt(false, time.Time{}, time.Time{}) != 250*time.Millisecond {
		t.Fatal("idle")
	}
	if sampleGapAt(true, time.Time{}, time.Time{}) != 2*time.Millisecond {
		t.Fatal("stale")
	}
	now := time.Unix(1000, 0)
	if sampleGapAt(false, now, now.Add(time.Second)) != 2*time.Millisecond {
		t.Fatal("fast window")
	}
	if sampleGapAt(false, now, now.Add(-time.Millisecond)) != 250*time.Millisecond {
		t.Fatal("after fast window")
	}
}

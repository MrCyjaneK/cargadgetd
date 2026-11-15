package session

import (
	"strings"
	"testing"
)

func TestParseSensorTypesVarint(t *testing.T) {
	sc := []byte{0x08, 10, 0x08, 13}
	ch := encodeLenKV(2, sc)
	got := parseSensorTypes(ch)
	if len(got) != 2 || got[0] != 10 || got[1] != 13 {
		t.Fatalf("%v", got)
	}
}

func TestParseSensorTypesNested(t *testing.T) {
	s10 := encodeLenKV(1, []byte{0x08, 10})
	s13 := encodeLenKV(1, []byte{0x08, 13})
	sc := append(s10, s13...)
	ch := encodeLenKV(2, sc)
	got := parseSensorTypes(ch)
	if len(got) != 2 || got[0] != SensorNightData || got[1] != SensorDrivingStatus {
		t.Fatalf("%v", got)
	}
}

func TestFormatSensorBatch(t *testing.T) {
	night := []byte{0x52, 0x02, 0x08, 0x00}
	drive := []byte{0x6a, 0x02, 0x08, 0x00}
	s := formatSensorBatch(append(night, drive...))
	if !strings.Contains(s, "NightData=day") || !strings.Contains(s, "DrivingStatus=UNRESTRICTED") {
		t.Fatalf("%q", s)
	}
}

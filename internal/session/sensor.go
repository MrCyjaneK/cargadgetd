package session

import (
	"fmt"
	"strings"
)

// GAL SensorType (AACS Sensor.proto).
const (
	SensorLocation      = 1
	SensorCompass       = 2
	SensorCarSpeed      = 3
	SensorRpm           = 4
	SensorOdometer      = 5
	SensorFuelLevel     = 6
	SensorParkingBrake  = 7
	SensorGear          = 8
	SensorDiagnostics   = 9
	SensorNightData     = 10
	SensorEnvironment   = 11
	SensorHVAC          = 12
	SensorDrivingStatus = 13
	SensorDeadReckoning = 14
	SensorPassenger     = 15
	SensorDoor          = 16
	SensorLight         = 17
	SensorTire          = 18
	SensorAccel         = 19
	SensorGyro          = 20
	SensorGPS           = 21
)

func sensorTypeName(t int) string {
	switch t {
	case SensorLocation:
		return "Location"
	case SensorCompass:
		return "Compass"
	case SensorCarSpeed:
		return "CarSpeed"
	case SensorRpm:
		return "Rpm"
	case SensorOdometer:
		return "Odometer"
	case SensorFuelLevel:
		return "FuelLevel"
	case SensorParkingBrake:
		return "ParkingBrake"
	case SensorGear:
		return "Gear"
	case SensorDiagnostics:
		return "Diagnostics"
	case SensorNightData:
		return "NightData"
	case SensorEnvironment:
		return "Environment"
	case SensorHVAC:
		return "HVAC"
	case SensorDrivingStatus:
		return "DrivingStatus"
	case SensorDeadReckoning:
		return "DeadReckoning"
	case SensorPassenger:
		return "Passenger"
	case SensorDoor:
		return "Door"
	case SensorLight:
		return "Light"
	case SensorTire:
		return "Tire"
	case SensorAccel:
		return "Accel"
	case SensorGyro:
		return "Gyro"
	case SensorGPS:
		return "GPS"
	default:
		return fmt.Sprintf("type=%d", t)
	}
}

func sensorBatchFieldName(fn int) string {
	if fn >= 1 && fn <= 21 {
		return sensorTypeName(fn)
	}
	return fmt.Sprintf("field%d", fn)
}

func formatSensorTypes(types []int) string {
	if len(types) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for i, t := range types {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d:%s", t, sensorTypeName(t))
	}
	return b.String()
}

func drivingStatusName(v int) string {
	if v == 0 {
		return "UNRESTRICTED"
	}
	var parts []string
	if v&1 != 0 {
		parts = append(parts, "NO_VIDEO")
	}
	if v&2 != 0 {
		parts = append(parts, "NO_KEYBOARD")
	}
	if v&4 != 0 {
		parts = append(parts, "NO_VOICE")
	}
	if v&8 != 0 {
		parts = append(parts, "NO_CONFIG")
	}
	if v&16 != 0 {
		parts = append(parts, "LIMIT_MESSAGE")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, "|")
}

// parseSensorTypes reads SensorChannel from a Channel blob (field 2).
// Accepts repeated int32 sensors=1 and nested Sensor { type=1 }.
func parseSensorTypes(ch []byte) []int {
	sc := lengthField(ch, 2)
	if len(sc) == 0 {
		sc = ch
	}
	var out []int
	seen := map[int]bool{}
	add := func(t int) {
		if t <= 0 || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	i := 0
	for i < len(sc) {
		tag, n := readVarint(sc[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(sc[i:])
			i += n2
			if fn == 1 {
				add(int(v))
			}
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(sc[i:])
		i += n2
		if i+int(l) > len(sc) {
			break
		}
		sub := sc[i : i+int(l)]
		i += int(l)
		if fn == 1 {
			if t, ok := protoVarintField(sub, 1); ok {
				add(int(t))
			}
		}
	}
	return out
}

func formatProtoInner(b []byte) string {
	var parts []string
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(b[i:])
			i += n2
			if fn == 1 {
				parts = append(parts, fmt.Sprintf("%d", v))
			} else {
				parts = append(parts, fmt.Sprintf("f%d=%d", fn, v))
			}
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
		parts = append(parts, fmt.Sprintf("f%d=%x", fn, sub))
	}
	if len(parts) == 0 && len(b) > 0 {
		return fmt.Sprintf("%x", b)
	}
	return strings.Join(parts, ",")
}

func formatSensorBatch(b []byte) string {
	if len(b) == 0 {
		return "(empty)"
	}
	var parts []string
	i := 0
	for i < len(b) {
		tag, n := readVarint(b[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		name := sensorBatchFieldName(fn)
		if wt == 0 {
			v, n2 := readVarint(b[i:])
			i += n2
			if fn == SensorDrivingStatus {
				parts = append(parts, fmt.Sprintf("%s=%s", name, drivingStatusName(int(v))))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%d", name, v))
			}
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
		inner := formatProtoInner(sub)
		if fn == SensorDrivingStatus {
			if v, ok := protoVarintField(sub, 1); ok {
				inner = drivingStatusName(int(v))
			}
		} else if fn == SensorNightData {
			if v, ok := protoVarintField(sub, 1); ok {
				if v != 0 {
					inner = "night"
				} else {
					inner = "day"
				}
			}
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, inner))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%x", b)
	}
	return strings.Join(parts, " ")
}

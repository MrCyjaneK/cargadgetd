package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
	"github.com/cargadgetd/cargadgetd/internal/rgh"
	"github.com/cargadgetd/cargadgetd/internal/videoenc"
)

const (
	settingBool   = 0
	settingEnum   = 1
	settingInt    = 2
	settingString = 3
)

// coreSpecs is the daemon YAML this library advertises. Plugin-owned keys
// stay on those plugins. The settings UI reads cargadgetd_app_daemon_settings_*.
type coreSpec struct {
	label   string
	kind    int
	enums   string
	enumsFn func() string
	accept  func(string) bool
	get     func(coreState) string
	set     func(string) error
}

var coreSpecs = []coreSpec{
	{label: "Shell", kind: settingString, get: func(s coreState) string { return s.shellID }, set: saveShell},
	{label: "Shell drop frames", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.dropFrames) }, set: saveInt(0, 1000, "shell", "drop_frames")},
	{label: "Audio enabled", kind: settingBool, get: boolBit(func(s coreState) bool { return s.audio }), set: saveBit(saveAudio)},
	{label: "Force audio primes", kind: settingBool, get: boolBit(func(s coreState) bool { return s.forcePrime }), set: saveBit(saveForcePrime)},
	{label: "Audio type", kind: settingEnum, enums: "auto,media,system,guidance,telephony", get: func(s coreState) string { return s.audioType }, set: saveAudioType},
	{label: "Audio channels", kind: settingEnum, enums: "auto,mono,stereo", get: func(s coreState) string { return channelLabel(s.channels) }, set: saveChannels},
	{label: "Audio tick ms", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.tickMs) }, set: saveInt(0, 1000, "audio", "tick_ms")},
	{label: "Audio config index", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.audioIndex) }, set: saveInt(-1, 32, "audio", "config_index")},
	{label: "Video encoder", kind: settingEnum, enumsFn: func() string { return videoenc.Titles() }, accept: func(v string) bool {
		_, ok := videoenc.Parse(v)
		return ok
	}, get: func(s coreState) string { return videoenc.Title(s.encoder) }, set: saveEncoder},
	{label: "Video size", kind: settingEnum, enums: "auto,800x480,1280x720,1920x1080,2560x1440,3840x2160", get: func(s coreState) string { return s.videoSize }, set: saveVideoSize},
	{label: "Frame ms", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.frameMs) }, set: func(v string) error {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return errString("frame_ms must be 16-200")
		}
		return saveFrame(n)
	}},
	{label: "Live video", kind: settingBool, get: boolBit(func(s coreState) bool { return s.live }), set: saveBit(saveLive)},
	{label: "Video config index", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.videoIndex) }, set: saveInt(-1, 32, "video", "config_index")},
	{label: "USB media gap ms", kind: settingInt, get: func(s coreState) string { return strconv.Itoa(s.gap) }, set: saveInt(0, 100, "usb", "media_gap_ms")},
	{label: "USB pcm dump", kind: settingString, get: func(s coreState) string { return s.pcm }, set: saveQuoted("usb", "pcm_dump")},
	{label: "USB ffs diag", kind: settingString, get: func(s coreState) string { return s.ffs }, set: saveQuoted("usb", "ffs_diag")},
	{label: "Strict protocol", kind: settingBool, get: boolBit(func(s coreState) bool { return s.strict }), set: saveBit(saveStrict)},
	{label: "Unhandled log", kind: settingString, get: func(s coreState) string { return s.unhandled }, set: saveQuoted("apps", "unhandled_log")},
	{label: "Loopback", kind: settingBool, get: boolBit(func(s coreState) bool { return s.loopback }), set: saveBit(saveLoopback)},
	{label: "Music sink", kind: settingString, get: func(s coreState) string { return s.sink }, set: saveSink},
	{label: "Music player", kind: settingString, get: func(s coreState) string { return s.player }, set: saveQuoted("music", "player")},
	{label: "GPS", kind: settingEnum, enums: "GeoClue,Mock", get: func(s coreState) string { return gpsLabel(s.gps) }, set: saveGPS},
	{label: "Transport", kind: settingEnum, enumsFn: func() string { return cfg.TransportTitles() }, accept: func(v string) bool {
		_, ok := cfg.ParseTransport(v)
		return ok
	}, get: func(s coreState) string { return cfg.TransportTitle(s.transport) }, set: saveTransport},
	{label: "Head unit", kind: settingEnum, enumsFn: peerEnums, accept: peerChoice, get: func(s coreState) string { return peerChoiceLabel(s.peer) }, set: savePeer},
	{label: "Listen", kind: settingString, get: func(s coreState) string { return s.listen }, set: saveListen},
	{label: "Control", kind: settingString, get: func(s coreState) string { return s.control }, set: saveControl},
	{label: "Certs", kind: settingString, get: func(s coreState) string { return s.certs }, set: saveCerts},
	{label: "Gadget path", kind: settingString, get: func(s coreState) string { return s.gadgetPath }, set: saveGadgetPath},
	{label: "Gadget bin", kind: settingString, get: func(s coreState) string { return s.gadgetBin }, set: saveQuoted("gadget", "bin")},
	{label: "First run", kind: settingBool, get: boolBit(func(s coreState) bool { return s.firstRun }), set: saveBit(saveFirstRun)},
}

func saveFirstRun(on bool) error { return patchBool("daemon", "first_run", on) }

func boolBit(get func(coreState) bool) func(coreState) string {
	return func(s coreState) string {
		if get(s) {
			return "1"
		}
		return "0"
	}
}

func saveBit(save func(bool) error) func(string) error {
	return func(v string) error {
		return save(v == "1")
	}
}

func saveForcePrime(on bool) error { return patchBool("audio", "force_prime", on) }
func saveLive(on bool) error       { return patchBool("video", "live", on) }
func saveStrict(on bool) error     { return patchBool("apps", "strict", on) }

func patchBool(section, key string, on bool) error {
	val := "false"
	if on {
		val = "true"
	}
	return patchSectionKey(userConfigPath(), section, key, val)
}

func saveInt(min, max int, section, key string) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < min || n > max {
			return errString(key + " out of range")
		}
		return patchSectionKey(userConfigPath(), section, key, strconv.Itoa(n))
	}
}

func saveQuoted(section, key string) func(string) error {
	return func(v string) error {
		return patchSectionKey(userConfigPath(), section, key, strconv.Quote(strings.TrimSpace(v)))
	}
}

func saveAudioType(v string) error {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "media", "system", "guidance", "telephony", "auto":
		return patchSectionKey(userConfigPath(), "audio", "type", strconv.Quote(strings.ToLower(strings.TrimSpace(v))))
	default:
		return errString("audio type must be auto, media, system, guidance, or telephony")
	}
}

func channelLabel(n int) string {
	switch n {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	default:
		return "auto"
	}
}

func saveChannels(v string) error {
	var n int
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "auto", "0":
		n = 0
	case "mono", "1":
		n = 1
	case "stereo", "2":
		n = 2
	default:
		return errString("channels must be auto, mono, or stereo")
	}
	return patchSectionKey(userConfigPath(), "audio", "channels", strconv.Itoa(n))
}

func saveVideoSize(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "auto", "800x480", "1280x720", "1920x1080", "2560x1440", "3840x2160":
		return patchSectionKey(userConfigPath(), "video", "size", strconv.Quote(v))
	default:
		return errString("video size must be auto or WxH")
	}
}

// packagedGadget is the setuid helper that can restore g1 and start usb-moded.
const packagedGadget = "/usr/share/cargadgetd/bin/cargadgetd-gadget"

func saveTransport(v string) error {
	mode, ok := cfg.ParseTransport(v)
	if !ok {
		return errString("unknown transport")
	}
	if err := patchSectionKey(userConfigPath(), "daemon", "transport", strconv.Quote(mode)); err != nil {
		return err
	}
	if _, err := os.Stat(packagedGadget); err != nil {
		return nil
	}
	switch mode {
	case cfg.TransportWireless:
		// Stop the AOA waiter before restoring the gadget. Leaving it up
		// would poll the config and keep the CPU out of deep idle.
		if err := runGadget("-aoa-service", "stop"); err != nil {
			return err
		}
		return runGadget("-usb-off")
	case cfg.TransportWPP:
		// The accessory gadget stays down. Restarting the unit is what
		// brings Bluetooth up and starts the dialer.
		if err := runGadget("-usb-off"); err != nil {
			return err
		}
		return runGadget("-aoa-service", "restart")
	default:
		return runGadget("-aoa-service", "start")
	}
}

func runGadget(args ...string) error {
	out, err := exec.Command(packagedGadget, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return errString(err.Error() + " (" + msg + ")")
	}
	return nil
}

func saveListen(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "ffs:///dev/usb-ffs/aa"
	}
	return patchSectionKey(userConfigPath(), "daemon", "listen", strconv.Quote(v))
}

func saveControl(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "tcp://127.0.0.1:9277"
	}
	return patchSectionKey(userConfigPath(), "daemon", "control", strconv.Quote(v))
}

func saveCerts(v string) error {
	return patchSectionKey(userConfigPath(), "daemon", "certs", strconv.Quote(rgh.Canonical(v)))
}

func saveGadgetPath(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "/sys/kernel/config/usb_gadget/aa"
	}
	return patchSectionKey(userConfigPath(), "gadget", "path", strconv.Quote(v))
}

func saveSink(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "sink.null"
	}
	return patchSectionKey(userConfigPath(), "music", "sink", strconv.Quote(v))
}

func coreAt(i int) (coreSpec, bool) {
	if i < 0 || i >= len(coreSpecs) {
		return coreSpec{}, false
	}
	return coreSpecs[i], true
}

func coreSettingsCount() int { return len(coreSpecs) }

func coreSettingsLabel(i int) string {
	s, ok := coreAt(i)
	if !ok {
		return ""
	}
	return s.label
}

func coreSettingsType(i int) int {
	s, ok := coreAt(i)
	if !ok {
		return -1
	}
	return s.kind
}

func coreSettingsEnums(i int) string {
	s, ok := coreAt(i)
	if !ok {
		return ""
	}
	if s.enumsFn != nil {
		return s.enumsFn()
	}
	return s.enums
}

func coreSettingsGetBool(i int) int {
	s, ok := coreAt(i)
	if !ok || s.kind != settingBool {
		return 0
	}
	if s.get(loadCore()) == "1" {
		return 1
	}
	return 0
}

func coreSettingsGetEnum(i int) string {
	s, ok := coreAt(i)
	if !ok || s.kind != settingEnum {
		return ""
	}
	return s.get(loadCore())
}

func coreSettingsGetInt(i int) int {
	s, ok := coreAt(i)
	if !ok || s.kind != settingInt {
		return 0
	}
	n, _ := strconv.Atoi(s.get(loadCore()))
	return n
}

func coreSettingsGetString(i int) string {
	s, ok := coreAt(i)
	if !ok || s.kind != settingString {
		return ""
	}
	return s.get(loadCore())
}

func coreFail(msg string) int {
	coreSettingsErr = msg
	return 1
}

var coreSettingsErr string

func coreClear() { coreSettingsErr = "" }

func coreSettingsSetBool(i, v int) int {
	coreClear()
	s, ok := coreAt(i)
	if !ok || s.kind != settingBool {
		return coreFail("not a bool")
	}
	bit := "0"
	if v != 0 {
		bit = "1"
	}
	if err := s.set(bit); err != nil {
		return coreFail(err.Error())
	}
	return 0
}

func coreSettingsSetEnum(i int, v string) int {
	coreClear()
	s, ok := coreAt(i)
	if !ok || s.kind != settingEnum {
		return coreFail("not an enum")
	}
	if s.accept != nil {
		if !s.accept(v) {
			return coreFail("unknown value")
		}
	} else if !enumHas(coreSettingsEnums(i), v) {
		return coreFail("unknown value")
	}
	if err := s.set(v); err != nil {
		return coreFail(err.Error())
	}
	return 0
}

func coreSettingsSetInt(i, v int) int {
	coreClear()
	s, ok := coreAt(i)
	if !ok || s.kind != settingInt {
		return coreFail("not an int")
	}
	if err := s.set(strconv.Itoa(v)); err != nil {
		return coreFail(err.Error())
	}
	return 0
}

func coreSettingsSetString(i int, v string) int {
	coreClear()
	s, ok := coreAt(i)
	if !ok || s.kind != settingString {
		return coreFail("not a string")
	}
	if err := s.set(v); err != nil {
		return coreFail(err.Error())
	}
	return 0
}

func coreSettingsError() string { return coreSettingsErr }

func enumHas(list, v string) bool {
	for _, t := range strings.Split(list, ",") {
		if strings.TrimSpace(t) == v {
			return true
		}
	}
	return false
}

package loopback

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

const pactlSinkInputs = `
Sink Input #2
	Driver: protocol-native.c
	Owner Module: 12
	Client: 8
	Sink: 0
	Sample Specification: s16le 1ch 48000Hz
	Corked: no
	Mute: no
	Volume: mono: 32768 /  50% / -18.06 dB
	Properties:
		application.process.binary = "ngfd"
		media.role = "event"
		policy.group = "event"
Sink Input #5
	Driver: protocol-native.c
	Owner Module: 12
	Client: 32
	Sink: 0
	Sample Specification: s16le 2ch 48000Hz
	Channel Map: front-left,front-right
	Corked: no
	Mute: no
	Volume: front-left: 26214 /  40% / -23.88 dB,   front-right: 26214 /  40% / -23.88 dB
	Properties:
		media.name = "Playback Stream"
		application.name = "AppSupport"
		application.process.binary = "appsupportaudio"
		media.role = "x-maemo"
		policy.group = "player"
Sink Input #6
	Driver: protocol-native.c
	Sink: 0
	Corked: yes
	Properties:
		policy.group = "player"
		media.role = "x-maemo"
`

const pactlShortSinks = `
0	sink.primary_output	module-droid-card.c	s16le 2ch 48000Hz	RUNNING
1	sink.null	module-null-sink.c	s16le 2ch 48000Hz	IDLE
`

func TestParseSinkInputsAndPickPlayer(t *testing.T) {
	in := parseSinkInputs(pactlSinkInputs)
	if len(in) != 3 {
		t.Fatalf("inputs %d: %+v", len(in), in)
	}
	if in[0].Group != "event" || in[0].ID != "2" {
		t.Fatalf("event %#v", in[0])
	}
	if in[1].Group != "player" || in[1].Corked != "no" || in[1].Sink != "0" {
		t.Fatalf("player %#v", in[1])
	}
	if in[1].Volume != "40% 40%" || in[1].Mute != "no" {
		t.Fatalf("player vol %#v", in[1])
	}
	if in[0].Volume != "50%" || in[0].Mute != "no" {
		t.Fatalf("event vol %#v", in[0])
	}
	if in[1].Spec != "s16le 2ch 48000Hz" {
		t.Fatalf("player spec %q", in[1].Spec)
	}
	if in[2].Corked != "yes" {
		t.Fatalf("corked %#v", in[2])
	}

	id, sink, ok := pickPlayer(in)
	if !ok || id != "5" || sink != "0" {
		t.Fatalf("pick %q %q %v", id, sink, ok)
	}

	// Do not key off application names - a corked player must lose to the live one.
	id, _, ok = pickPlayer([]sinkInput{
		{ID: "9", Sink: "0", Corked: "yes", Group: "player"},
		{ID: "5", Sink: "0", Corked: "no", Group: "player"},
	})
	if !ok || id != "5" {
		t.Fatalf("live player %q", id)
	}
	if _, _, ok := pickPlayer([]sinkInput{
		{ID: "2", Group: "event", Corked: "no"},
	}); ok {
		t.Fatal("event is not a player")
	}
}

func TestParsePactlInfoAndHealHelpers(t *testing.T) {
	sink, src := parsePactlInfo("Server Name: pulseaudio\nDefault Sink: sink.null\nDefault Source: source.null\n")
	if sink != "sink.null" || src != "source.null" {
		t.Fatalf("%q %q", sink, src)
	}
	sinks := parseShortSinks(pactlShortSinks)
	if g := preferredHardwareSink(sinks, "sink.null", "sink.null"); g != "sink.primary_output" {
		t.Fatalf("hw from null default %q", g)
	}
	if g := preferredHardwareSink(sinks, "sink.primary_output", "sink.null"); g != "sink.primary_output" {
		t.Fatalf("keep %q", g)
	}
	srcs := parseShortSinks("2\tsource.primary_input\tmodule-droid-card.c\ts16le 2ch 48000Hz\tRUNNING\n4\tsource.null\tmodule-null-source.c\ts16le 2ch 48000Hz\tSUSPENDED\n")
	if g := preferredHardwareSource(srcs, "source.null"); g != "source.primary_input" {
		t.Fatalf("src %q", g)
	}
	ids := inputsOnSink([]sinkInput{
		{ID: "5", Sink: "1", Group: "player"},
		{ID: "2", Sink: "0", Group: "event"},
	}, sinks, "sink.null")
	if len(ids) != 1 || ids[0] != "5" {
		t.Fatalf("%v", ids)
	}
}

func TestMonitorSource(t *testing.T) {
	srcs := parseShortSinks("0\tsink.primary_output.monitor\tmodule-droid-card.c\ts16le 2ch 48000Hz\tRUNNING\n1\tsink.null.monitor\tmodule-null-sink.c\ts16le 2ch 48000Hz\tIDLE\n2\tsource.primary_input\tmodule-droid-card.c\ts16le 2ch 48000Hz\tRUNNING\n")
	if g, ok := monitorSource(srcs, "sink.null"); !ok || g != "sink.null.monitor" {
		t.Fatalf("%q %v", g, ok)
	}
	if g, ok := monitorSource(map[string]string{"0": "source.primary_input"}, "sink.null"); ok || g != "" {
		t.Fatalf("must not fall back to mic %q", g)
	}
	if isMonitorName("source.primary_input") || isMonitorName("source.null") {
		t.Fatal("inputs are not monitors")
	}
	if !isMonitorName("sink.null.monitor") {
		t.Fatal("monitor")
	}
}

func TestParseSourceOutputs(t *testing.T) {
	text := `
Source Output #8
	Driver: protocol-native.c
	Client: 219
	Source: 2
	Sample Specification: s16le 2ch 48000Hz
	Corked: no
	Properties:
		application.name = "parec"
		application.process.id = "51910"
		policy.group = "player"
`
	got := parseSourceOutputs(text)
	if len(got) != 1 || got[0].ID != "8" || got[0].Source != "2" || got[0].PID != "51910" || got[0].App != "parec" {
		t.Fatalf("%+v", got)
	}
	if got[0].Spec != "s16le 2ch 48000Hz" {
		t.Fatalf("spec %q", got[0].Spec)
	}
	so, ok := pickSourceOutputByPID(got, 51910)
	if !ok || so.ID != "8" {
		t.Fatalf("%v %+v", ok, so)
	}
	if _, ok := pickSourceOutputByPID(got, 1); ok {
		t.Fatal("wrong pid")
	}
}

func TestPulseChildEnvOverridesSource(t *testing.T) {
	t.Setenv("PULSE_SOURCE", "source.primary_input")
	t.Setenv("PULSE_SINK", "sink.primary_output")
	t.Setenv("PULSE_SERVER", "unix:/run/user/100000/pulse/native")
	env := pulseChildEnv("sink.null.monitor")
	var src, sink string
	nSrc := 0
	for _, e := range env {
		if strings.HasPrefix(e, "PULSE_SOURCE=") {
			nSrc++
			src = strings.TrimPrefix(e, "PULSE_SOURCE=")
		}
		if strings.HasPrefix(e, "PULSE_SINK=") {
			sink = strings.TrimPrefix(e, "PULSE_SINK=")
		}
	}
	if nSrc != 1 || src != "sink.null.monitor" {
		t.Fatalf("PULSE_SOURCE count=%d val=%q", nSrc, src)
	}
	if sink != "" {
		t.Fatalf("inherited PULSE_SINK %q", sink)
	}
}

func TestParseShortSinksAndHardware(t *testing.T) {
	sinks := parseShortSinks(pactlShortSinks)
	if sinks["0"] != "sink.primary_output" || sinks["1"] != "sink.null" {
		t.Fatalf("%v", sinks)
	}
	if g := hardwareSink(sinks, "0", "sink.null"); g != "sink.primary_output" {
		t.Fatalf("from idx %q", g)
	}
	if g := hardwareSink(sinks, "1", "sink.null"); g != "sink.primary_output" {
		t.Fatalf("already on null %q", g)
	}
}

func TestPcmS16LE(t *testing.T) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b[0:], 0x0001)
	binary.LittleEndian.PutUint16(b[2:], 0xfffe) // -2
	got := pcmS16LE(b)
	if len(got) != 2 || got[0] != 1 || got[1] != -2 {
		t.Fatalf("%v", got)
	}
}

func TestPulseEnvForcesCLocale(t *testing.T) {
	t.Setenv("LANG", "pl_PL.UTF-8")
	t.Setenv("LC_ALL", "pl_PL.UTF-8")
	t.Setenv("LANGUAGE", "pl")
	got := map[string]int{}
	for _, e := range pulseEnv() {
		k, _, ok := strings.Cut(e, "=")
		if !ok || !isLocaleEnv(k+"=") {
			continue
		}
		got[e]++
	}
	if got["LANG=C"] != 1 || got["LC_ALL=C"] != 1 || got["LC_MESSAGES=C"] != 1 {
		t.Fatalf("%v", got)
	}
	if got["LANG=pl_PL.UTF-8"] != 0 || got["LANGUAGE=pl"] != 0 || got["LC_ALL=pl_PL.UTF-8"] != 0 {
		t.Fatalf("locale leaked: %v", got)
	}
}

func TestLoopbackEnabled(t *testing.T) {
	cfg.SetForTest(t, nil)
	if !loopbackEnabled() {
		t.Fatal("default on")
	}
	t.Run("loopback off", func(t *testing.T) {
		cfg.SetForTest(t, func(c *cfg.Config) { c.Music.Loopback = false })
		if loopbackEnabled() {
			t.Fatal("loopback false disables")
		}
	})
	t.Run("audio off", func(t *testing.T) {
		cfg.SetForTest(t, func(c *cfg.Config) { c.Audio.Enabled = false })
		if loopbackEnabled() {
			t.Fatal("audio.enabled false disables")
		}
	})
}

func TestLoopSinkOverride(t *testing.T) {
	cfg.SetForTest(t, nil)
	if loopSinkName() != "sink.null" {
		t.Fatal(loopSinkName())
	}
	cfg.SetForTest(t, func(c *cfg.Config) { c.Music.Sink = "sink.foo" })
	if loopSinkName() != "sink.foo" {
		t.Fatal(loopSinkName())
	}
}

func TestNewPulseLoopbackHonorsDisable(t *testing.T) {
	cfg.SetForTest(t, func(c *cfg.Config) { c.Music.Loopback = false })
	if newPulseLoopback() != nil {
		t.Fatal("disabled must be nil")
	}
}

func TestVolumePercentsAndRestoreCmds(t *testing.T) {
	got := volumePercents("Volume: front-left: 26214 /  40% / -23.88 dB,   front-right: 26214 /  40% / -23.88 dB")
	if got != "40% 40%" {
		t.Fatalf("stereo %q", got)
	}
	if g := volumePercents("Volume: mono: 32768 /  50% / -18.06 dB"); g != "50%" {
		t.Fatalf("mono %q", g)
	}
	snap := parseVolSnapFile("40% 40%\nno\n")
	if snap.vol != "40% 40%" || snap.mute != "no" {
		t.Fatalf("%+v", snap)
	}
	cmds := sinkInputVolumeCmds("5", snap)
	if len(cmds) != 2 {
		t.Fatalf("%v", cmds)
	}
	if strings.Join(cmds[0], " ") != "set-sink-input-volume 5 40% 40%" {
		t.Fatalf("vol %v", cmds[0])
	}
	if strings.Join(cmds[1], " ") != "set-sink-input-mute 5 no" {
		t.Fatalf("mute %v", cmds[1])
	}
	if sinkInputVolumeCmds("", snap) != nil {
		t.Fatal("empty id")
	}
}

func TestPlayersAwayIncludesCorked(t *testing.T) {
	sinks := parseShortSinks(pactlShortSinks)
	away := playersAway(parseSinkInputs(pactlSinkInputs), sinks, "sink.null")
	if len(away) != 2 || away[0].ID != "5" || away[1].ID != "6" {
		t.Fatalf("%+v", away)
	}
	onNull := []sinkInput{{ID: "5", Sink: "1", Group: "player", Corked: "no"}}
	if got := playersAway(onNull, sinks, "sink.null"); len(got) != 0 {
		t.Fatalf("already on null: %+v", got)
	}
	if got := playersAway([]sinkInput{{ID: "2", Sink: "0", Group: "event"}}, sinks, "sink.null"); len(got) != 0 {
		t.Fatalf("event: %+v", got)
	}
}

func TestSinkInputEvent(t *testing.T) {
	if !sinkInputEvent("Event 'new' on sink-input #12") || !sinkInputEvent("Event 'change' on sink-input #3") {
		t.Fatal("sink-input event")
	}
	if sinkInputEvent("Event 'new' on client #1") || sinkInputEvent("Event 'remove' on source-output #4") {
		t.Fatal("other event")
	}
}

func TestVolumeStateFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	writeVolumeState(volSnap{vol: "35%", mute: "yes"})
	got := readVolumeState()
	if got.vol != "35%" || got.mute != "yes" {
		t.Fatalf("%+v", got)
	}
	clearVolumeState()
	if got := readVolumeState(); got.vol != "" || got.mute != "" {
		t.Fatalf("cleared %+v", got)
	}
}

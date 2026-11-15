package session

import (
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func TestAudioWantFromConfig(t *testing.T) {
	cfg.SetForTest(t, nil)
	if audioWant() != -1 || audioTypeWant() != 0 {
		t.Fatalf("default idx=%d type=%d", audioWant(), audioTypeWant())
	}
	cfg.SetForTest(t, func(c *cfg.Config) {
		c.Audio.ConfigIndex = 2
		c.Audio.Type = "system"
	})
	if audioWant() != 2 || audioTypeWant() != AudioTypeSystem {
		t.Fatalf("idx=%d type=%d", audioWant(), audioTypeWant())
	}
}

func TestParseAudioConfig(t *testing.T) {
	// 48000, 16bit, stereo (hupeer audioChannel)
	c := parseAudioConfig([]byte{0x08, 0x80, 0xf7, 0x02, 0x10, 16, 0x18, 2})
	if c.Rate != 48000 || c.Bits != 16 || c.Chans != 2 {
		t.Fatalf("got %+v", c)
	}
	c = parseAudioConfig([]byte{0x08, 0x80, 0x7d, 0x10, 16, 0x18, 1}) // 16000 16 1
	if c.Rate != 16000 || c.Chans != 1 {
		t.Fatalf("16k mono %+v", c)
	}
}

func TestParseMediaAudioConfigs(t *testing.T) {
	ac16 := []byte{0x08, 0x80, 0x7d, 0x10, 16, 0x18, 1}
	ac48 := []byte{0x08, 0x80, 0xf7, 0x02, 0x10, 16, 0x18, 2}
	mc := []byte{0x08, 1, 0x10, 1} // Audio + Media
	mc = append(mc, encodeLenKV(3, ac16)...)
	mc = append(mc, encodeLenKV(3, ac48)...)
	if parseMediaAudioType(mc) != AudioTypeMedia {
		t.Fatalf("type %d", parseMediaAudioType(mc))
	}
	got := parseMediaAudioConfigs(mc)
	if len(got) != 2 || got[0].Rate != 16000 || got[1].Rate != 48000 || got[1].Chans != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestSelectAudioConfigPrefers48kStereo(t *testing.T) {
	cfgs := []audioCfg{
		{Rate: 16000, Bits: 16, Chans: 1},
		{Rate: 48000, Bits: 16, Chans: 2},
	}
	c, i := selectAudioConfig(cfgs, -1)
	if i != 1 || c.Rate != 48000 || c.Chans != 2 {
		t.Fatalf("got %+v idx=%d", c, i)
	}
	c, i = selectAudioConfig(cfgs, 0)
	if i != 0 || c.Rate != 16000 {
		t.Fatalf("env idx0 %+v idx=%d", c, i)
	}
	c, i = selectAudioConfig(nil, -1)
	if i != 1 || c.Rate != 48000 || c.Chans != 2 {
		t.Fatalf("empty %+v idx=%d", c, i)
	}
}

func TestSelectAudioSinkFallsToSystemWhenMediaIs16k(t *testing.T) {
	sinks := []audioSink{
		{Ch: 2, Type: AudioTypeMedia, Cfgs: []audioCfg{{Rate: 16000, Bits: 16, Chans: 1}}},
		{Ch: 3, Type: AudioTypeSystem, Cfgs: []audioCfg{{Rate: 48000, Bits: 16, Chans: 2}}},
	}
	s, c, i := selectAudioSink(sinks, 0, -1)
	if s.Type != AudioTypeSystem || s.Ch != 3 || c.Rate != 48000 || c.Chans != 2 || i != 0 {
		t.Fatalf("auto: %+v %+v idx=%d", s, c, i)
	}
	s, c, _ = selectAudioSink(sinks, AudioTypeMedia, -1)
	if s.Type != AudioTypeMedia || c.Rate != 16000 {
		t.Fatalf("force media: %+v %+v", s, c)
	}
}

func TestSelectAudioSinkKeepsMediaWhen48k(t *testing.T) {
	sinks := []audioSink{
		{Ch: 4, Type: AudioTypeMedia, Cfgs: []audioCfg{
			{Rate: 16000, Bits: 16, Chans: 1},
			{Rate: 48000, Bits: 16, Chans: 1},
		}},
		{Ch: 5, Type: AudioTypeSystem, Cfgs: []audioCfg{{Rate: 48000, Bits: 16, Chans: 2}}},
	}
	s, c, i := selectAudioSink(sinks, 0, -1)
	if s.Type != AudioTypeMedia || s.Ch != 4 || c.Rate != 48000 || c.Chans != 1 || i != 1 {
		t.Fatalf("%+v %+v idx=%d", s, c, i)
	}
	s, c, i = selectAudioSink(sinks, AudioTypeSystem, -1)
	if s.Type != AudioTypeSystem || s.Ch != 5 || c.Chans != 2 {
		t.Fatalf("force system: %+v %+v idx=%d", s, c, i)
	}
}

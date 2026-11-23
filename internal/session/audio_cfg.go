package session

import (
	"fmt"
	"strings"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

const (
	AudioTypeGuidance  = 1
	AudioTypeSystem    = 2
	AudioTypeMedia     = 3
	AudioTypeTelephony = 4
)

type audioCfg struct {
	Rate, Bits, Chans int
}

func (c audioCfg) String() string {
	return fmt.Sprintf("%dHz/%dbit/%dch", c.Rate, c.Bits, c.Chans)
}

func audioTypeName(t int) string {
	switch t {
	case AudioTypeGuidance:
		return "guidance"
	case AudioTypeSystem:
		return "system"
	case AudioTypeMedia:
		return "media"
	case AudioTypeTelephony:
		return "telephony"
	default:
		return fmt.Sprintf("type=%d", t)
	}
}

func formatAudioCfgs(cfgs []audioCfg) string {
	if len(cfgs) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for i, c := range cfgs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d:%s", i, c)
	}
	return b.String()
}

func parseAudioConfig(ac []byte) audioCfg {
	var c audioCfg
	i := 0
	for i < len(ac) {
		tag, n := readVarint(ac[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt != 0 {
			break
		}
		v, n2 := readVarint(ac[i:])
		i += n2
		switch fn {
		case 1:
			c.Rate = int(v)
		case 2:
			c.Bits = int(v)
		case 3:
			c.Chans = int(v)
		}
	}
	return c
}

func parseMediaAudioType(media []byte) int {
	i := 0
	for i < len(media) {
		tag, n := readVarint(media[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			v, n2 := readVarint(media[i:])
			i += n2
			if fn == 2 {
				return int(v)
			}
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(media[i:])
		i += n2
		i += int(l)
	}
	return 0
}

func parseMediaAudioConfigs(media []byte) []audioCfg {
	var out []audioCfg
	i := 0
	for i < len(media) {
		tag, n := readVarint(media[i:])
		if n == 0 {
			break
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt == 0 {
			_, n2 := readVarint(media[i:])
			i += n2
			continue
		}
		if wt != 2 {
			break
		}
		l, n2 := readVarint(media[i:])
		i += n2
		sub := media[i : i+int(l)]
		i += int(l)
		if fn != 3 {
			continue
		}
		c := parseAudioConfig(sub)
		if c.Rate > 0 {
			if c.Bits < 1 {
				c.Bits = 16
			}
			if c.Chans < 1 {
				c.Chans = 1
			}
			out = append(out, c)
		}
	}
	return out
}

func selectAudioConfig(cfgs []audioCfg, wantIdx int) (audioCfg, int) {
	fallback := audioCfg{Rate: 48000, Bits: 16, Chans: 2}
	if wantIdx >= 0 && wantIdx < len(cfgs) {
		return cfgs[wantIdx], wantIdx
	}
	if len(cfgs) == 0 {
		idx := 1
		if wantIdx >= 0 {
			idx = wantIdx
		}
		return fallback, idx
	}
	for i, c := range cfgs {
		if c.Rate == 48000 && c.Chans >= 2 {
			return c, i
		}
	}
	for i, c := range cfgs {
		if c.Rate == 48000 {
			return c, i
		}
	}
	return cfgs[0], 0
}

type audioSink struct {
	Ch   uint8
	Type int
	Cfgs []audioCfg
}

func audioMaxRate(cfgs []audioCfg) int {
	best := 0
	for _, c := range cfgs {
		if c.Rate > best {
			best = c.Rate
		}
	}
	return best
}

func sinkByType(sinks []audioSink, typ int) (audioSink, bool) {
	for _, s := range sinks {
		if s.Type == typ {
			return s, true
		}
	}
	return audioSink{}, false
}

// selectAudioSink picks which HU audio channel to open.
// wantType 0 = auto: MEDIA if it has 48 kHz, else SYSTEM if that has 48 kHz, else MEDIA.
func selectAudioSink(sinks []audioSink, wantType, wantIdx int) (audioSink, audioCfg, int) {
	pick := func(s audioSink) (audioSink, audioCfg, int) {
		c, i := selectAudioConfig(s.Cfgs, wantIdx)
		return s, c, i
	}
	if wantType != 0 {
		if s, ok := sinkByType(sinks, wantType); ok {
			return pick(s)
		}
	}
	media, hasMedia := sinkByType(sinks, AudioTypeMedia)
	sys, hasSys := sinkByType(sinks, AudioTypeSystem)
	if hasMedia && audioMaxRate(media.Cfgs) >= 48000 {
		return pick(media)
	}
	if hasSys && audioMaxRate(sys.Cfgs) >= 48000 {
		return pick(sys)
	}
	if hasMedia {
		return pick(media)
	}
	if hasSys {
		return pick(sys)
	}
	if len(sinks) > 0 {
		return pick(sinks[0])
	}
	c, i := selectAudioConfig(nil, wantIdx)
	return audioSink{Type: AudioTypeMedia}, c, i
}

func audioWant() int {
	n := cfg.Get().Audio.ConfigIndex
	if n < 0 {
		return -1
	}
	return n
}

func audioTypeWant() int {
	switch strings.ToLower(strings.TrimSpace(cfg.Get().Audio.Type)) {
	case "guidance":
		return AudioTypeGuidance
	case "system":
		return AudioTypeSystem
	case "media":
		return AudioTypeMedia
	case "telephony":
		return AudioTypeTelephony
	default:
		return 0
	}
}

// AudioFormat is the HU audio config selected at SDR (rate, channels, config_index).
func (m *MD) AudioFormat() (rate, chans, cfgIdx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rate, chans, cfgIdx = m.audioRate, m.audioChans, m.audioCfgIdx
	if rate < 1 {
		rate = 48000
	}
	if chans < 1 {
		chans = 2
	}
	return rate, chans, cfgIdx
}

func (m *MD) AudioKind() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return audioTypeName(m.audioType)
}

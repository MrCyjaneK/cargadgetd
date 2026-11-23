package session

// Media playback channel messages. LIVI's MediaPlaybackStatusMessageId:
// status 0x8001, input 0x8002, metadata 0x8003.
const (
	MediaPlaybackStatus   = 0x8001
	MediaPlaybackInput    = 0x8002
	MediaPlaybackMetadata = 0x8003
)

// GAL MediaPlaybackState.
const (
	PlaybackStopped = 1
	PlaybackPlaying = 2
	PlaybackPaused  = 3
)

// PlaybackMeta is the part of a now-playing snapshot that is sent as metadata.
type PlaybackMeta struct {
	Title, Artist, Album string
	Duration             int
	Art                  []byte
}

func (a PlaybackMeta) Same(b PlaybackMeta) bool {
	return a.Title == b.Title && a.Artist == b.Artist && a.Album == b.Album &&
		a.Duration == b.Duration && bytesEqual(a.Art, b.Art)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// EncodePlaybackMetadata is MediaPlaybackMetadataNotification.
// Field 2 is the artist and field 3 is the album: that is how LIVI reads them.
func EncodePlaybackMetadata(title, artist, album string, art []byte, duration int) []byte {
	var b []byte
	b = append(b, encodeLenDelim(1, []byte(title))...)
	b = append(b, encodeLenDelim(2, []byte(artist))...)
	b = append(b, encodeLenDelim(3, []byte(album))...)
	if len(art) > 0 {
		b = append(b, encodeLenDelim(4, art)...)
	}
	if duration > 0 {
		b = append(b, encodeProtoVarint(6, uint64(duration))...)
	}
	return b
}

// EncodePlaybackStatus is MediaPlaybackStatusNotification.
func EncodePlaybackStatus(state int, app string, position int) []byte {
	var b []byte
	if state > 0 {
		b = append(b, encodeProtoVarint(1, uint64(state))...)
	}
	b = append(b, encodeLenDelim(2, []byte(app))...)
	if position < 0 {
		position = 0
	}
	b = append(b, encodeProtoVarint(3, uint64(position))...)
	return b
}

// MediaPlaybackCh is the advertised media-playback service, or 0.
func (m *MD) MediaPlaybackCh() uint8 {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.side {
		if s.kind == "media_playback" {
			return s.id
		}
	}
	return 0
}

// SendPlaybackMetadata writes a metadata notification on the media-playback
// channel. A head unit that did not advertise the channel is a no-op.
func (m *MD) SendPlaybackMetadata(title, artist, album string, art []byte, duration int) error {
	ch := m.MediaPlaybackCh()
	if ch == 0 {
		return nil
	}
	return m.sendEnc(ch, 0, MediaPlaybackMetadata, EncodePlaybackMetadata(title, artist, album, art, duration))
}

// SendPlaybackStatus writes a status notification on the media-playback channel.
func (m *MD) SendPlaybackStatus(state int, app string, position int) error {
	ch := m.MediaPlaybackCh()
	if ch == 0 {
		return nil
	}
	return m.sendEnc(ch, 0, MediaPlaybackStatus, EncodePlaybackStatus(state, app, position))
}

// clusterPadAction names LIVI's InstrumentClusterAction values.
// LIVI numbers them VALUE_0 through VALUE_7. These names are the published
// InstrumentClusterAction enum: up, down, left, right, enter, back, call.
func clusterPadAction(v int) (string, bool) {
	switch v {
	case 1:
		return "dpad_up", true
	case 2:
		return "dpad_down", true
	case 3:
		return "dpad_left", true
	case 4:
		return "dpad_right", true
	case 5:
		return "dpad_click", true
	case 6:
		return "back", true
	case 7:
		return "call", true
	default:
		return "", false
	}
}

func (m *MD) onClusterPad(channel uint8, body []byte) {
	v, ok := protoVarintField(body, 1)
	if !ok {
		m.log.Printf("media playback input ch=%d missing action body=%x", channel, body)
		return
	}
	name, known := clusterPadAction(int(v))
	if !known {
		m.log.Printf("media playback input ch=%d action=%d", channel, v)
		return
	}
	m.log.Printf("media playback input ch=%d action=%d %s", channel, v, name)
	m.emit(Event{Type: "huinput", Channel: channel, Payload: []byte(name)})
}

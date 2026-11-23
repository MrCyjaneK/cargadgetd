package session

import (
	"io"
	"log"
	"testing"
)

func TestVideoFocusProjectedNoInputKeepsStreaming(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))

	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x01, 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("focus=1 should start video")
	}
	select {
	case <-m.WaitVideoStarted():
	default:
		t.Fatal("focus=1 should release the video loop")
	}
	start := recvStartInd(t, m)

	// DHU sends this while the projection stays up (input focus elsewhere).
	// Pausing here reopened Codec2 on every return to focus=1.
	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x04, 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("focus=4 should keep video streaming")
	}
	if msg, ok := tryRecv(m); ok {
		t.Fatalf("focus=4 while streaming enqueued %x", msg.payload)
	}

	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x01, 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("focus=1 while streaming should stay up")
	}
	if msg, ok := tryRecv(m); ok {
		t.Fatalf("repeat focus=1 enqueued %x", msg.payload)
	}

	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x02}); err != nil {
		t.Fatal(err)
	}
	if m.VideoStreaming() {
		t.Fatal("focus=2 (native) should pause video")
	}

	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x01, 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("focus=1 after native loss should resume")
	}
	again := recvStartInd(t, m)
	if string(again) != string(start) {
		t.Fatalf("resume StartInd %x, first was %x", again, start)
	}
}

func TestAllowVideoSendsStart(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	m.AllowVideo()
	if !m.VideoStreaming() {
		t.Fatal("timeout start should mark video streaming")
	}
	select {
	case <-m.WaitVideoStarted():
	default:
		t.Fatal("StartInd should release the video loop")
	}
	recvStartInd(t, m)
	m.AllowVideo()
	if msg, ok := tryRecv(m); ok {
		t.Fatalf("second AllowVideo enqueued %x", msg.payload)
	}
}

func TestVideoFocus4StartsWhenIdle(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	if err := m.onVideoFocus(ChanVideo, []byte{0x08, 0x04, 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !m.VideoStreaming() {
		t.Fatal("focus=4 with nothing playing should start video")
	}
	recvStartInd(t, m)
}

func recvStartInd(t *testing.T, m *MD) []byte {
	t.Helper()
	msg, ok := tryRecv(m)
	if !ok {
		t.Fatal("missing StartInd")
	}
	if len(msg.payload) < 2 || msg.payload[0] != 0x80 || msg.payload[1] != 0x01 {
		t.Fatalf("payload %x, want MediaStartInd", msg.payload)
	}
	return msg.payload
}

func tryRecv(m *MD) (outMsg, bool) {
	select {
	case msg := <-m.sendQ:
		return msg, true
	default:
		return outMsg{}, false
	}
}

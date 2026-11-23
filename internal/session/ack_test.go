package session

import (
	"errors"
	"io"
	"log"
	"testing"
	"time"
)

func TestProtocol61SkipsMediaAckCredit(t *testing.T) {
	m := NewMD(nil, nil, log.New(io.Discard, "", 0))
	m.SetUSBBulk(true)
	m.protoMaj, m.protoMin = 6, 1
	m.videoStreaming.Store(true)
	if m.videoAckRequired() || m.audioAckRequired() {
		t.Fatal("protocol 6.1 must not wait for video or audio MediaAck")
	}
	if err := m.ReserveVideoCredit(0); err != nil {
		t.Fatal(err)
	}
	if err := m.ReserveVideoCredit(0); err != nil {
		t.Fatal(err)
	}
	if m.VideoInFlight() != 0 {
		t.Fatalf("in_flight=%d, ackless video must not take a credit", m.VideoInFlight())
	}
	if m.GiveUpVideoAck(time.Millisecond) {
		t.Fatal("ackless session must not abandon video credit")
	}
	if m.LastVideoAckAgeLabel() != "never" {
		t.Fatalf("ack age %q", m.LastVideoAckAgeLabel())
	}
	m.AbandonAudioCredit("test")
	if err := m.WaitAudioCredit(time.Millisecond); err != nil {
		t.Fatal(err)
	}

	m.protoMaj = 5
	if m.audioAckRequired() || !m.videoAckRequired() {
		t.Fatal("protocol 5.0 acks video and not audio")
	}
	m.protoMaj = 1
	if !m.videoAckRequired() || !m.audioAckRequired() {
		t.Fatal("protocol 1.x still acks both")
	}
	if err := m.ReserveVideoCredit(0); err != nil {
		t.Fatal(err)
	}
	if m.VideoInFlight() != 1 {
		t.Fatalf("in_flight=%d, want the single video credit", m.VideoInFlight())
	}
	if err := m.ReserveVideoCredit(0); !errors.Is(err, ErrNoVideoCredit) {
		t.Fatalf("second reserve: %v", err)
	}
}

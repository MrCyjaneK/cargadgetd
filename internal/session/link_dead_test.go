package session

import (
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"
)

type hangConn struct {
	writeErr error
	once     sync.Once
	wake     chan struct{}
}

func newHangConn(err error) *hangConn {
	return &hangConn{writeErr: err, wake: make(chan struct{})}
}

func (h *hangConn) Read(p []byte) (int, error) {
	<-h.wake
	return 0, io.EOF
}

func (h *hangConn) Write(p []byte) (int, error) {
	return 0, h.writeErr
}

func (h *hangConn) Close() error {
	h.once.Do(func() { close(h.wake) })
	return nil
}

func TestTCPWriteErrorEndsSession(t *testing.T) {
	rw := newHangConn(errors.New("connection timed out"))
	md := NewMD(rw, nil, log.New(io.Discard, "", 0))
	done := make(chan error, 1)
	go func() { done <- md.Run() }()

	deadline := time.Now().Add(2 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		err = md.enqueueTimeout(2, 0, []byte{0x00, 0x01}, 200*time.Millisecond)
		if err != nil && !errors.Is(err, errSendTimeout) && !errors.Is(err, errSendQueueFull) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err == nil || errors.Is(err, errSendTimeout) || errors.Is(err, errSendQueueFull) {
		t.Fatalf("write did not fail the send: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run returned nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session still running after the projection write failed")
	}
}

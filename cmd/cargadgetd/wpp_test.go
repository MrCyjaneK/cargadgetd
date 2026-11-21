package main

import (
	"net"
	"testing"
	"time"
)

func TestGALStopUnblocksServe(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	var g galSession
	returned := make(chan struct{})
	if err := g.start(a, func() {
		buf := make([]byte, 1)
		_, _ = a.Read(buf)
		close(returned)
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		g.stop()
		close(done)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("serve still blocked after bluetooth session ended")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not return")
	}
}

func TestGALWaitLeavesProjectionUp(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	var g galSession
	if err := g.start(a, func() {
		buf := make([]byte, 1)
		_, _ = a.Read(buf)
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		g.wait()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("wait returned while the projection was still reading")
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := b.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return when the projection finished")
	}
}

func TestWPPNext(t *testing.T) {
	if wait, n := wppNext(wppSession, 2); wait || n != 0 {
		t.Fatalf("session wait=%v misses=%d", wait, n)
	}
	if wait, n := wppNext(wppHandshake, 0); wait || n != 1 {
		t.Fatalf("first handshake wait=%v misses=%d", wait, n)
	}
	if wait, n := wppNext(wppHandshake, 1); wait || n != 2 {
		t.Fatalf("second handshake wait=%v misses=%d", wait, n)
	}
	if wait, n := wppNext(wppHandshake, 2); !wait || n != 0 {
		t.Fatalf("third handshake wait=%v misses=%d", wait, n)
	}
	if wait, n := wppNext(wppAbsent, 2); !wait || n != 0 {
		t.Fatalf("absent wait=%v misses=%d", wait, n)
	}
}

func TestGALStopRejectsLateServe(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	var g galSession
	g.stop()
	called := false
	if err := g.start(a, func() { called = true }); err == nil {
		t.Fatal("expected closed session")
	}
	if called {
		t.Fatal("serve ran after bluetooth session ended")
	}
	if _, err := a.Write([]byte{1}); err == nil {
		t.Fatal("connection still open")
	}
}

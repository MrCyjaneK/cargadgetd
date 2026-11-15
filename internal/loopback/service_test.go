package loopback

import (
	"testing"
	"time"
)

type fakeLoop struct {
	started chan struct{}
	halted  bool
}

func (f *fakeLoop) Run(stop <-chan struct{}) {
	close(f.started)
	<-stop
}

func (f *fakeLoop) Halt() {
	f.halted = true
}

func TestServiceStartsAndStopHalts(t *testing.T) {
	f := &fakeLoop{started: make(chan struct{})}
	s := startWith(f)
	select {
	case <-f.started:
	case <-time.After(time.Second):
		t.Fatal("loop did not start")
	}
	s.Stop()
	if !f.halted {
		t.Fatal("stop did not halt")
	}
	s.Stop()
}

func TestServiceDisabledIsIdle(t *testing.T) {
	s := startWith(nil)
	s.Stop()
}

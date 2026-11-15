package loopback

import (
	"sync"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

// Service keeps phone playback on the capture sink for this Android Auto
// session and feeds PCM to play. It stays up while a song is paused, and
// moves the next track as soon as Pulse creates it.
type Service struct {
	once sync.Once
	stop chan struct{}
	loop interface {
		Run(stop <-chan struct{})
		Halt()
	}
}

// Start begins loopback for this session. play receives 48 kHz stereo s16le.
// A nil play or a disabled config still returns a Service whose Stop is safe.
func Start(play func(pcm []int16, nframes, channels, rate int)) *Service {
	setCapturePlay(play)
	setCaptureDump(cfg.Get().USB.PCMDump)
	// A nil *pulseLoopback must not be stored in the interface, or Stop
	// would call Run on it.
	if lb := newPulseLoopback(); lb != nil {
		return startWith(lb)
	}
	return startWith(nil)
}

func startWith(loop interface {
	Run(stop <-chan struct{})
	Halt()
}) *Service {
	s := &Service{stop: make(chan struct{}), loop: loop}
	if loop == nil {
		return s
	}
	go loop.Run(s.stop)
	return s
}

// Stop restores the phone sink. It is safe to call more than once.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		if s.loop != nil {
			s.loop.Halt()
		}
	})
}

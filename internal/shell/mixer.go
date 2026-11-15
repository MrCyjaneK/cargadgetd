package shell

import "sync"

// Mixer queues plugin PCM into the USB output format.
// Play is a FIFO: consecutive chunks from one stream are concatenated, not
// overlapping-added. (Saturating-add of each play() made music loopback into
// mush — every ~42ms chunk was mixed on top of the previous one.)
type Mixer struct {
	mu      sync.Mutex
	cond    *sync.Cond
	rate    int
	chans   int
	pending []int16
	onPlay  func()
	primed  bool // true after we have held one Mix-chunk ahead
	closed  bool // PlayBlocking returns; session capture is stopping
}

func NewMixer(rate, chans int) *Mixer {
	if rate <= 0 {
		rate = 48000
	}
	if chans < 1 {
		chans = 2
	}
	m := &Mixer{rate: rate, chans: chans}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *Mixer) maxPending() int {
	// Capture blocks at this ceiling (~400ms) so Pulse pauses the player
	// instead of this FIFO dropping samples. Mix still waits for two full
	// chunks before it starts, and never emits a short one.
	n := m.rate * m.chans * 400 / 1000
	if n < m.chans*256 {
		n = m.chans * 256
	}
	return n
}

// Play queues PCM. nframes is frames (not samples). Converted to host rate/chans.
func (m *Mixer) Play(pcm []int16, nframes, channels, rate int) {
	m.enqueue(pcm, nframes, channels, rate, false)
}

// PlayBlocking queues PCM and waits when the FIFO is at its ceiling.
// Loopback capture uses this so a slow USB ack pauses the phone player
// instead of throwing away samples. Unblock wakes a waiter during shutdown.
func (m *Mixer) PlayBlocking(pcm []int16, nframes, channels, rate int) {
	m.enqueue(pcm, nframes, channels, rate, true)
}

// Unblock releases PlayBlocking waiters. Later PlayBlocking calls return
// without queueing. Safe to call more than once.
func (m *Mixer) Unblock() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.closed = true
	m.cond.Broadcast()
	m.mu.Unlock()
}

func (m *Mixer) enqueue(pcm []int16, nframes, channels, rate int, block bool) {
	if m == nil || nframes <= 0 || channels < 1 {
		return
	}
	if rate <= 0 {
		rate = m.rate
	}
	host := resample(pcm, nframes, channels, rate, m.rate, m.chans)
	if len(host) == 0 {
		return
	}
	m.mu.Lock()
	if block {
		for !m.closed && len(m.pending)+len(host) > m.maxPending() {
			m.cond.Wait()
		}
		if m.closed {
			m.mu.Unlock()
			return
		}
	}
	m.pending = append(m.pending, host...)
	if !block {
		if extra := len(m.pending) - m.maxPending(); extra > 0 {
			m.pending = m.pending[extra:]
		}
	}
	on := m.onPlay
	m.mu.Unlock()
	if on != nil {
		on() // must return quickly — no USB/OpenAudio on this stack
	}
}

// SetOnPlay is invoked after PCM is queued. The callback must not block
// (host OpenAudio runs on its own goroutine).
func (m *Mixer) SetOnPlay(fn func()) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.onPlay = fn
	m.mu.Unlock()
}

// PCM16LE encodes interleaved s16le.
func PCM16LE(s []int16) []byte {
	out := make([]byte, len(s)*2)
	for i, v := range s {
		out[i*2] = byte(v)
		out[i*2+1] = byte(v >> 8)
	}
	return out
}

func (m *Mixer) Pending() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending) > 0
}

// Mix writes one host-format chunk into dst (zeros dst first).
// Starts only after two full chunks are queued (one to send, one ahead).
// Never copies a partial chunk — a short buffer is silence, not a click.
func (m *Mixer) Mix(dst []int16) {
	for i := range dst {
		dst[i] = 0
	}
	if m == nil || len(dst) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	need := len(dst)
	if !m.primed {
		if len(m.pending) < need*2 {
			return
		}
		m.primed = true
	}
	if len(m.pending) < need {
		if len(m.pending) == 0 {
			m.primed = false
		}
		return
	}
	copy(dst, m.pending[:need])
	m.pending = m.pending[need:]
	m.cond.Broadcast()
}

func resample(in []int16, nframes, inCh, inRate, outRate, outCh int) []int16 {
	if nframes*inCh > len(in) {
		nframes = len(in) / inCh
	}
	if nframes <= 0 {
		return nil
	}
	if inRate == outRate && inCh == outCh {
		out := make([]int16, nframes*outCh)
		copy(out, in[:nframes*inCh])
		return out
	}
	outFrames := int(int64(nframes) * int64(outRate) / int64(inRate))
	if outFrames < 1 {
		outFrames = 1
	}
	out := make([]int16, outFrames*outCh)
	for of := 0; of < outFrames; of++ {
		src := of * inRate / outRate
		if src >= nframes {
			src = nframes - 1
		}
		for c := 0; c < outCh; c++ {
			ic := c
			if ic >= inCh {
				ic = 0
			}
			out[of*outCh+c] = in[src*inCh+ic]
		}
	}
	return out
}

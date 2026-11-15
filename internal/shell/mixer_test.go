package shell

import (
	"testing"
	"time"
)

func TestMixerPlayIsSequentialNotOverlapping(t *testing.T) {
	m := NewMixer(48000, 1)
	m.Play([]int16{20000, 20000}, 2, 1, 48000)
	m.Play([]int16{20000, 20000}, 2, 1, 48000)
	dst := make([]int16, 2)
	m.Mix(dst)
	if dst[0] != 20000 || dst[1] != 20000 {
		t.Fatalf("first chunk overlapped? %v", dst)
	}
	m.Mix(dst)
	if dst[0] != 20000 || dst[1] != 20000 {
		t.Fatalf("second chunk %v", dst)
	}
	m.Mix(dst)
	if dst[0] != 0 {
		t.Fatalf("drained: %v", dst)
	}
}

func TestMixerStreamChunksStayInOrder(t *testing.T) {
	m := NewMixer(48000, 2)
	a := make([]int16, 4)
	b := make([]int16, 4)
	for i := range a {
		a[i] = 100
		b[i] = 200
	}
	m.Play(a, 2, 2, 48000)
	m.Play(b, 2, 2, 48000)
	dst := make([]int16, 4)
	m.Mix(dst)
	for i, v := range dst {
		if v != 100 {
			t.Fatalf("chunk0[%d]=%d", i, v)
		}
	}
	m.Mix(dst)
	for i, v := range dst {
		if v != 200 {
			t.Fatalf("chunk1[%d]=%d", i, v)
		}
	}
}

func TestMixerDropsOldestWhenAhead(t *testing.T) {
	m := NewMixer(8000, 1)
	n := m.maxPending()
	pcm := make([]int16, n+100)
	for i := range pcm {
		pcm[i] = int16(i)
	}
	m.Play(pcm, len(pcm), 1, 8000)
	if m.Pending() && len(m.pending) > n {
		t.Fatalf("pending %d max %d", len(m.pending), n)
	}
}

func TestMixerStereoFromMono(t *testing.T) {
	m := NewMixer(48000, 2)
	m.Play([]int16{1000, 2000}, 2, 1, 48000)
	m.Play([]int16{3000, 4000}, 2, 1, 48000)
	dst := make([]int16, 4)
	m.Mix(dst)
	if dst[0] != 1000 || dst[1] != 1000 || dst[2] != 2000 || dst[3] != 2000 {
		t.Fatalf("got %v", dst)
	}
}

func TestMixerPrebuffersOneChunk(t *testing.T) {
	m := NewMixer(48000, 1)
	m.Play([]int16{1, 2}, 2, 1, 48000)
	dst := make([]int16, 2)
	m.Mix(dst)
	if dst[0] != 0 || dst[1] != 0 {
		t.Fatalf("emitted before one-ahead: %v", dst)
	}
	m.Play([]int16{3, 4}, 2, 1, 48000)
	m.Mix(dst)
	if dst[0] != 1 || dst[1] != 2 {
		t.Fatalf("first chunk %v", dst)
	}
	if len(m.pending) != 2 || m.pending[0] != 3 || m.pending[1] != 4 {
		t.Fatalf("ahead leftover %v", m.pending)
	}
}

func TestPlayBlockingWaitsUntilMixFreesRoom(t *testing.T) {
	m := NewMixer(1000, 1)
	fill := make([]int16, m.maxPending())
	for i := range fill {
		fill[i] = 1
	}
	m.PlayBlocking(fill, len(fill), 1, 1000)
	done := make(chan struct{})
	go func() {
		m.PlayBlocking([]int16{9}, 1, 1, 1000)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("blocked play returned before Mix")
	case <-time.After(40 * time.Millisecond):
	}
	dst := make([]int16, 1)
	m.Mix(dst)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("play stayed blocked after Mix")
	}
	if dst[0] != 1 {
		t.Fatalf("mix %v", dst)
	}
}

func TestPlayBlockingUnblockDropsWaiter(t *testing.T) {
	m := NewMixer(1000, 1)
	fill := make([]int16, m.maxPending())
	m.PlayBlocking(fill, len(fill), 1, 1000)
	done := make(chan struct{})
	go func() {
		m.PlayBlocking([]int16{9}, 1, 1, 1000)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("returned before Unblock")
	case <-time.After(40 * time.Millisecond):
	}
	m.Unblock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Unblock did not wake play")
	}
	m.mu.Lock()
	n := len(m.pending)
	m.mu.Unlock()
	if n != len(fill) {
		t.Fatalf("queued the dropped chunk: %d", n)
	}
}

func TestMixerDoesNotEmitPartialChunk(t *testing.T) {
	m := NewMixer(48000, 1)
	m.Play([]int16{1, 2, 3}, 3, 1, 48000)
	dst := make([]int16, 4)
	m.Mix(dst)
	for i, v := range dst {
		if v != 0 {
			t.Fatalf("partial Mix dst[%d]=%d", i, v)
		}
	}
	if len(m.pending) != 3 {
		t.Fatalf("dropped leftover: %d", len(m.pending))
	}
	m.Play([]int16{4, 5, 6, 7, 8}, 5, 1, 48000)
	m.Mix(dst)
	if dst[0] != 1 || dst[3] != 4 {
		t.Fatalf("full chunk %v", dst)
	}
}

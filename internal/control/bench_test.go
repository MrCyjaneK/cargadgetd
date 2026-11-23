package control

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/livevid"
)

func resetBench(t *testing.T) {
	t.Helper()
	benchMu.Lock()
	benchRunning = false
	benchDiscard = false
	benchStatus = ""
	benchRaw = nil
	benchMu.Unlock()
}

func waitBench(t *testing.T, pred func() bool) {
	t.Helper()
	dead := time.Now().Add(2 * time.Second)
	for time.Now().Before(dead) {
		if pred() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the benchmark")
}

func TestBenchClearDropsFinishedRun(t *testing.T) {
	resetBench(t)
	prev := benchRun
	hold := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(hold) }) }
	benchRun = func(encoder string, status func(string), live func(livevid.Cell), cell func(livevid.Cell)) livevid.Result {
		<-hold
		return livevid.Result{Encoder: encoder, Cells: []livevid.Cell{{
			W: 8, H: 8, Mode: "30", Error: "no video/avc encoder",
		}}}
	}
	t.Cleanup(func() {
		release()
		waitBench(t, func() bool { return !BenchRunning() })
		benchRun = prev
		resetBench(t)
	})
	if msg := BenchStart("droidmedia"); msg != "" {
		t.Fatal(msg)
	}
	waitBench(t, func() bool { return BenchRunning() })
	BenchClear()
	release()
	waitBench(t, func() bool { return !BenchRunning() })
	var view benchView
	if err := json.Unmarshal([]byte(BenchResult()), &view); err != nil {
		t.Fatal(err)
	}
	if view.CanSelect || len(view.Rows) != 0 || view.Encoder != "" {
		t.Fatalf("discarded run published %+v", view)
	}
}

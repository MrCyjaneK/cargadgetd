package session

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

// Optional HU→MD capture (UNHANDLED, sensor, input, session control).
// Enable with apps.unhandled_log: ~/cargadgetd-unhandled.log (empty disables).
// Does not include MediaAck / video_tx / pings (those flood).

var (
	capMu   sync.Mutex
	capFile *os.File
	capPath string
	capFail atomic.Bool
	capOnce atomic.Bool
)

func capturePath() string {
	return cfg.ExpandPath(cfg.Get().Apps.UnhandledLog)
}

func captureEnabled() bool { return capturePath() != "" }

func captureLog(kind, msg string) {
	p := capturePath()
	if p == "" {
		return
	}
	capMu.Lock()
	defer capMu.Unlock()
	if capFile == nil || capPath != p {
		if capFile != nil {
			_ = capFile.Close()
			capFile = nil
		}
		capPath = p
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			if capFail.CompareAndSwap(false, true) {
				log.Printf("unhandled_log %s: %v", p, err)
			}
			return
		}
		capFile = f
		capFail.Store(false)
		fmt.Fprintf(capFile, "# cargadgetd unhandled GAL capture started %s path=%s\n",
			time.Now().UTC().Format(time.RFC3339Nano), p)
	}
	fmt.Fprintf(capFile, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), kind, msg)
}

func (m *MD) capturef(kind, format string, args ...any) {
	if !captureEnabled() {
		return
	}
	if capOnce.CompareAndSwap(false, true) {
		m.log.Printf("unhandled GAL log: %s", capturePath())
	}
	captureLog(kind, fmt.Sprintf(format, args...))
}

func formatInputCapture(body []byte) string {
	inner := formatProtoInner(body)
	if t, ok := ParseTouch(body); ok {
		if inner != "" {
			return fmt.Sprintf("touch=%d,%d action=%d proto=%s", t.X, t.Y, t.Action, inner)
		}
		return fmt.Sprintf("touch=%d,%d action=%d", t.X, t.Y, t.Action)
	}
	if inner == "" {
		return "(empty)"
	}
	return "proto=" + inner
}

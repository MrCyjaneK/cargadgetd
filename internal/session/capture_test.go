package session

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func resetCaptureForTest() {
	capMu.Lock()
	if capFile != nil {
		_ = capFile.Close()
		capFile = nil
	}
	capPath = ""
	capFail.Store(false)
	capOnce.Store(false)
	capMu.Unlock()
}

func TestUnhandledLogWritesSeparateFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gal.log")
	cfg.SetForTest(t, func(c *cfg.Config) { c.Apps.UnhandledLog = p })
	t.Cleanup(resetCaptureForTest)

	md := NewMD(nil, nil, log.New(io.Discard, "", 0))
	if err := md.unhandled("input", 3, 0x8005, []byte{0x08, 0x01}); err != nil {
		t.Fatal(err)
	}
	md.capturef("SENSOR", "NightData=day body=52020800")

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "UNHANDLED input ch=3 msg=0x8005") {
		t.Fatalf("missing UNHANDLED:\n%s", s)
	}
	if !strings.Contains(s, "SENSOR NightData=day") {
		t.Fatalf("missing SENSOR:\n%s", s)
	}
}

func TestUnhandledLogEmptyDisabled(t *testing.T) {
	cfg.SetForTest(t, nil)
	t.Cleanup(resetCaptureForTest)
	md := NewMD(nil, nil, log.New(io.Discard, "", 0))
	if err := md.unhandled("ch", 9, 0x0001, []byte{0x00}); err != nil {
		t.Fatal(err)
	}
	if captureEnabled() {
		t.Fatal("empty unhandled_log must disable capture")
	}
}

func TestFormatInputCaptureTouchAndProto(t *testing.T) {
	// TouchEvent field 3: location field 1 x=10 y=20, action field 3 = 0
	loc := encodeLenKV(1, append(encodeProtoVarint(1, 10), encodeProtoVarint(2, 20)...))
	te := append(loc, encodeProtoVarint(3, 0)...)
	body := encodeLenKV(3, te)
	got := formatInputCapture(body)
	if !strings.Contains(got, "touch=10,20") {
		t.Fatalf("%q", got)
	}
	btn := encodeLenKV(5, []byte{0x08, 0x57, 0x10, 0x01})
	got = formatInputCapture(btn)
	if !strings.Contains(got, "proto=") {
		t.Fatalf("want proto walk, got %q", got)
	}
}

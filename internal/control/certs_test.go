package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
	"github.com/cargadgetd/cargadgetd/internal/rgh"
)

func TestResetCertsLocation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := cfg.UserPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "video:\n  live: true\ndaemon:\n  listen: \"ffs:///dev/usb-ffs/aa\"\n  certs: \"/custom/certs\"\n  first_run: false\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configFiles
	configFiles = []string{path}
	t.Cleanup(func() { configFiles = prev })

	if err := resetCertsLocation(); err != nil {
		t.Fatal(err)
	}
	if got := readDaemon().Certs; got != rgh.ConfigDefault {
		t.Fatalf("certs %q", got)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, rgh.ConfigDefault) {
		t.Fatalf("config:\n%s", text)
	}
	if !strings.Contains(text, "first_run: false") || !strings.Contains(text, "live: true") || !strings.Contains(text, "listen:") {
		t.Fatalf("config:\n%s", text)
	}
}

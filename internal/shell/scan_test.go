package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDoesNotBlockOnOpen(t *testing.T) {
	orig := pluginOpener
	t.Cleanup(func() { pluginOpener = orig })

	started := make(chan struct{})
	block := make(chan struct{})
	pluginOpener = func(path string) (*App, error) {
		close(started)
		<-block
		return &App{ID: "hello", Name: "Hello"}, nil
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libcargadgetd_app_hello.so"), []byte("not-elf"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Bare names and shell plugins share the directory and are not apps.
	for _, name := range []string{"music.so", "notes.so", "libcargadgetd_app_foo-bar.so", "libcargadgetd_shell_basic.so"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not-elf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := New(800, 480, 48000, 2)
	begin := time.Now()
	ids := Load(s, []string{dir})
	if time.Since(begin) > 200*time.Millisecond {
		t.Fatalf("Load blocked for %v", time.Since(begin))
	}
	if len(ids) != 1 || ids[0] != "hello" {
		t.Fatalf("ids %v", ids)
	}
	if len(s.apps) != 0 {
		t.Fatalf("app visible before dlopen finished")
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("opener not started")
	}
	close(block)
	if !s.WaitLoad(2 * time.Second) {
		t.Fatal("WaitLoad timeout")
	}
	if s.apps[0].Name != "Hello" {
		t.Fatalf("attached name %q", s.apps[0].Name)
	}
}

func TestLoadMapsSettingsWithoutAttaching(t *testing.T) {
	origOpen, origSettings, origOnly := pluginOpener, settingsOpener, settingsOnlyCheck
	t.Cleanup(func() {
		pluginOpener, settingsOpener, settingsOnlyCheck = origOpen, origSettings, origOnly
	})

	var opened []string
	settingsOpener = func(path string) error {
		opened = append(opened, path)
		return nil
	}
	pluginOpener = func(path string) (*App, error) {
		t.Fatalf("settings library opened as an app: %s", path)
		return nil, nil
	}
	settingsOnlyCheck = func(path string) bool { return true }

	dir := t.TempDir()
	so := filepath.Join(dir, "libcargadgetd_app_daemon.so")
	if err := os.WriteFile(so, []byte("not-elf"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(32, 32, 48000, 2)
	if ids := Load(s, []string{dir}); len(ids) != 0 {
		t.Fatalf("loaded %v", ids)
	}
	if !s.WaitLoad(2 * time.Second) {
		t.Fatal("WaitLoad timeout")
	}
	if len(opened) != 1 || opened[0] != so {
		t.Fatalf("mapped %v", opened)
	}
	if len(s.apps) != 0 {
		t.Fatalf("attached %d", len(s.apps))
	}
}

func TestLoadStartsWhenAlreadyStarted(t *testing.T) {
	orig := pluginOpener
	t.Cleanup(func() { pluginOpener = orig })

	r := &fakeRunner{}
	pluginOpener = func(path string) (*App, error) {
		return &App{ID: "hello", Name: "Hello", runner: r}, nil
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libcargadgetd_app_hello.so"), []byte("not-elf"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(800, 480, 48000, 2)
	s.StartApps()
	Load(s, []string{dir})
	if !s.WaitLoad(2 * time.Second) {
		t.Fatal("WaitLoad timeout")
	}
	if !r.started {
		t.Fatal("Start not called on late attach")
	}
}

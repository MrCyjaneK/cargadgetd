//go:build linux && cgo

package shell

import (
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildCShared(t *testing.T, root, pkg, so string) {
	t.Helper()
	dir := root
	target := pkg
	// A plugin with its own go.mod (hello, area, music, shells, sample, sats) is not part of this module.
	if rel, ok := strings.CutPrefix(pkg, "./"); ok {
		nested := filepath.Join(root, rel)
		fi, err := os.Stat(nested)
		if err != nil || !fi.IsDir() {
			t.Skipf("%s lives in its own module", pkg)
		}
		if _, err := os.Stat(filepath.Join(nested, "go.mod")); err == nil {
			dir = nested
			target = "."
		}
	}
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", so, target)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("c-shared %s: %v\n%s", pkg, err, out)
	}
}

// waitShellFrame kicks one draw and copies it into dst once the shell goroutine publishes.
func waitShellFrame(t *testing.T, s *Shell, dst *image.RGBA) {
	t.Helper()
	gen := s.publishGen()
	s.Draw(dst, time.Now())
	if !s.waitPublished(gen, 2*time.Second) {
		t.Fatal("shell draw did not publish")
	}
	if hung, copied := s.takeFrame(dst); hung || !copied {
		t.Fatalf("shell frame missing hung=%v copied=%v", hung, copied)
	}
}

func openBasic(t *testing.T, s *Shell, root, dir string) {
	t.Helper()
	so := filepath.Join(dir, "libcargadgetd_shell_basic.so")
	buildCShared(t, root, "./shells/basic", so)
	if err := s.OpenShell(dir, "basic"); err != nil {
		t.Fatal(err)
	}
	if !s.WaitShell(30 * time.Second) {
		t.Fatal("shell init")
	}
}

func TestDlopenGoSharedHello(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	so := filepath.Join(dir, "libcargadgetd_app_hello.so")
	buildCShared(t, root, "./apps/helloworld", so)

	s := New(800, 480, 48000, 2)
	var played int
	s.Mixer().SetOnPlay(func() { played++ })
	openBasic(t, s, root, dir)
	ids := Load(s, []string{dir})
	if !s.WaitLoad(30 * time.Second) {
		t.Fatal("plugin load timeout")
	}
	if len(ids) != 1 || ids[0] != "hello" {
		t.Fatalf("loaded %v", ids)
	}
	if id, err := AppIDFromFile(so); err != nil || id != "hello" {
		t.Fatalf("AppIDFromFile=%q err=%v", id, err)
	}
	if s.apps[0].Name != "Hello" {
		t.Fatalf("name=%q", s.apps[0].Name)
	}
	if s.apps[0].Icon == nil || s.apps[0].Icon.Bounds().Dx() < 32 {
		t.Fatalf("missing embedded icon")
	}
	s.StartApps()
	defer s.Close()

	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	waitShellFrame(t, s, dst)
	// First cell center on an 800x480 canvas (basic shell TestFirstCellCenter800x480).
	s.Touch(182, 88, 1)
	s.Touch(182, 88, 0)
	waitShellFrame(t, s, dst)
	// Hello's button is the center of the content rect (rail is 96px).
	s.Touch(448, 240, 1)
	waitShellFrame(t, s, dst)
	if played < 1 {
		t.Fatalf("play() not called")
	}
	btn := color.RGBA{R: 0x3a, G: 0x7b, B: 0xd5, A: 0xff}
	if dst.RGBAAt(448, 240) != btn {
		t.Fatalf("button pixel %+v want pressed %+v", dst.RGBAAt(448, 240), btn)
	}
}

func TestDlopenHelloAndShells(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	buildCShared(t, root, "./apps/helloworld", filepath.Join(dir, "libcargadgetd_app_hello.so"))
	buildCShared(t, root, "./apps/shells", filepath.Join(dir, "libcargadgetd_app_shells.so"))

	s := New(1280, 720, 48000, 2)
	openBasic(t, s, root, dir)
	ids := Load(s, []string{dir})
	if !s.WaitLoad(30 * time.Second) {
		t.Fatal("plugin load timeout")
	}
	if len(ids) != 2 {
		t.Fatalf("loaded %v", ids)
	}
	got := map[string]bool{}
	for _, a := range s.apps {
		got[a.ID] = true
		if a.Icon == nil || a.Icon.Bounds().Dx() < 32 {
			t.Fatalf("%s missing icon", a.ID)
		}
	}
	if !got["hello"] || !got["shells"] {
		t.Fatalf("ids %v", ids)
	}
	s.StartApps()
	defer s.Close()
	dst := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	waitShellFrame(t, s, dst)
	if dst.RGBAAt(0, 0).R == 0 && dst.RGBAAt(0, 0).G == 0 && dst.RGBAAt(0, 0).B == 0 {
		t.Fatal("shell frame is still the init placeholder")
	}
}

func TestHostFrameMsFromSetFramePeriod(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	so := filepath.Join(dir, "libcargadgetd_app_hello.so")
	buildCShared(t, root, "./apps/helloworld", so)

	s := New(800, 480, 48000, 2)
	s.SetFramePeriod(50 * time.Millisecond)
	if ids := Load(s, []string{dir}); len(ids) != 1 {
		t.Fatalf("loaded %v", ids)
	}
	if !s.WaitLoad(30 * time.Second) {
		t.Fatal("plugin load timeout")
	}
	s.StartApps()
	defer s.Close()
	if got := hostFrameMs(); got != 50 {
		t.Fatalf("host frame_ms=%d", got)
	}
}

func TestDlopenLink(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	so := filepath.Join(dir, "libcargadgetd_app_link.so")
	buildCShared(t, root, "./apps/link", so)

	s := New(800, 480, 48000, 2)
	openBasic(t, s, root, dir)
	ids := Load(s, []string{dir})
	if !s.WaitLoad(30 * time.Second) {
		t.Fatal("plugin load timeout")
	}
	if len(ids) != 1 || ids[0] != "link" {
		t.Fatalf("ids %v", ids)
	}
	s.StartApps()
	defer s.Close()
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	s.Draw(dst, time.Now())
}

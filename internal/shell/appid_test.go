package shell

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseAppSymbol(t *testing.T) {
	id, m, ok := parseAppSymbol("cargadgetd_app_hello_draw")
	if !ok || id != "hello" || m != "draw" {
		t.Fatalf("hello_draw: %q %q %v", id, m, ok)
	}
	id, m, ok = parseAppSymbol("cargadgetd_app_my_app_resume")
	if !ok || id != "my_app" || m != "resume" {
		t.Fatalf("my_app_resume: %q %q %v", id, m, ok)
	}
	if _, _, ok := parseAppSymbol("cg_app_get"); ok {
		t.Fatal("old symbol should not parse")
	}
	if _, _, ok := parseAppSymbol("cargadgetd_app_hello"); ok {
		t.Fatal("missing method")
	}
	if _, _, ok := parseAppSymbol("cargadgetd_app_hello_huinput_playpause"); ok {
		t.Fatal("huinput must not parse as id/method")
	}
	if _, _, ok := parseAppSymbol("cargadgetd_app_hello_huinput_pause"); ok {
		t.Fatal("huinput_pause must not look like _pause")
	}
	if _, _, ok := parseAppSymbol("cargadgetd_app_music_settings_count"); ok {
		t.Fatal("settings_count must not parse as an app method")
	}
	if _, _, ok := parseAppSymbol("cargadgetd_app_music_settings_get_int"); ok {
		t.Fatal("settings_get_int must not parse as an app method")
	}
}

func TestAppIDFromSymbolNames(t *testing.T) {
	id, err := appIDFromSymbolNames([]string{
		"foo",
		"cargadgetd_app_hello_init",
		"cargadgetd_app_hello_draw",
		"cargadgetd_app_hello_name",
		"cargadgetd_app_hello_huinput_playpause",
		"cargadgetd_app_hello_huinput_pause",
	})
	if err != nil || id != "hello" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if _, err := appIDFromSymbolNames([]string{"cargadgetd_app_hello_init"}); err == nil {
		t.Fatal("draw required")
	}
	if _, err := appIDFromSymbolNames([]string{
		"cargadgetd_app_hello_draw",
		"cargadgetd_app_shells_draw",
	}); err == nil {
		t.Fatal("multiple ids")
	}
}

func TestSettingsOnlyNames(t *testing.T) {
	names := []string{
		"cargadgetd_app_daemon_name",
		"cargadgetd_app_daemon_settings_count",
		"cargadgetd_app_daemon_settings_label",
	}
	if !settingsOnlyNames("daemon", names) {
		t.Fatal("settings library")
	}
	names = append(names, "cargadgetd_app_daemon_draw")
	if settingsOnlyNames("daemon", names) {
		t.Fatal("an app with draw is not settings-only")
	}
	if settingsOnlyNames("hello", []string{"cargadgetd_app_hello_init"}) {
		t.Fatal("missing settings_count is not settings-only")
	}
}

func TestSettingsOnlyLibraryIsNotLoaded(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("cc")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "d.c")
	body := "const char *cargadgetd_app_daemon_name(void){return \"Cargadgetd\";}\n" +
		"int cargadgetd_app_daemon_settings_count(void){return 0;}\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	so := filepath.Join(dir, "libcargadgetd_app_daemon.so")
	cmd := exec.Command("cc", "-shared", "-fPIC", "-o", so, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cc -shared: %v\n%s", err, out)
	}
	if _, err := elf.Open(so); err != nil {
		t.Skip("shared library is not ELF")
	}
	if !settingsOnly(so) {
		t.Fatal("settings library looked like an app")
	}
	s := New(32, 32, 48000, 2)
	if ids := Load(s, []string{dir}); len(ids) != 0 {
		t.Fatalf("loaded %v", ids)
	}
}

func TestAppIDFromBasename(t *testing.T) {
	id, err := appIDFromBasename("/tmp/libcargadgetd_app_shells.so")
	if err != nil || id != "shells" {
		t.Fatalf("%q %v", id, err)
	}
	id, err = appIDFromBasename("/usr/lib/cargadgetd/plugins/libcargadgetd_app_hello.so")
	if err != nil || id != "hello" {
		t.Fatalf("hello: %q %v", id, err)
	}
	for _, name := range []string{
		"/tmp/shells.so",
		"/tmp/music.so",
		"/tmp/libcargadgetd_app_foo-bar.so",
		"/tmp/libcargadgetd_app_.so",
		"/usr/lib/cargadgetd/plugins/reversegearhead.so",
		"/usr/lib/cargadgetd/plugins/libcargadgetd_shell_basic.so",
	} {
		if _, err := appIDFromBasename(name); err == nil {
			t.Fatalf("%s should fail", name)
		}
	}
}

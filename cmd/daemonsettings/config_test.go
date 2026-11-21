package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreYAMLEdits(t *testing.T) {
	dir := t.TempDir()
	share := filepath.Join(dir, "share.yaml")
	user := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(share, []byte("video:\n  frame_ms: 40\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := "# keep\nvideo:\n  encoder: \"droidmedia\"\n  frame_ms: 33\n  size: \"auto\"\naudio:\n  # pcm\n  enabled: true\n  type: \"auto\"\nshell:\n  id: \"basic\"\nmusic:\n  loopback: true\n"
	if err := os.WriteFile(user, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configSearch = []string{share, user}
	configUser = user

	st := loadCore()
	if st.frameMs != 33 || st.encoder != "droidmedia" || !st.audio || st.shellID != "basic" || st.dropFrames != 20 {
		t.Fatalf("merged %+v", st)
	}
	if err := saveFrame(10); err == nil {
		t.Fatal("frame 10 accepted")
	}
	if err := saveShell("bad id"); err == nil {
		t.Fatal("shell with a space accepted")
	}
	if err := saveEncoder("x264"); err == nil {
		t.Fatal("encoder x264 accepted")
	}
	got, err := os.ReadFile(user)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != orig {
		t.Fatalf("rejected edits wrote the file:\n%s", got)
	}

	if err := saveFrame(50); err != nil {
		t.Fatal(err)
	}
	if err := saveShell("neon"); err != nil {
		t.Fatal(err)
	}
	if err := saveEncoder("software"); err != nil {
		t.Fatal(err)
	}
	if err := saveAudio(false); err != nil {
		t.Fatal(err)
	}
	if err := saveLoopback(false); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(user)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "# keep") || !strings.Contains(text, "# pcm") {
		t.Fatalf("comments dropped:\n%s", text)
	}
	if !strings.Contains(text, "size: \"auto\"") || !strings.Contains(text, "type: \"auto\"") {
		t.Fatalf("other keys dropped:\n%s", text)
	}
	if !strings.Contains(text, "loopback: false") {
		t.Fatalf("loopback not stored:\n%s", text)
	}
	if !strings.Contains(text, "frame_ms: 50") || !strings.Contains(text, "id: \"neon\"") || !strings.Contains(text, "encoder: \"software\"") || !strings.Contains(text, "enabled: false") {
		t.Fatalf("edits missing:\n%s", text)
	}
	st = loadCore()
	if st.frameMs != 50 || st.shellID != "neon" || st.encoder != "software" || st.audio || st.loopback {
		t.Fatalf("reloaded %+v", st)
	}

	shareGot, err := os.ReadFile(share)
	if err != nil {
		t.Fatal(err)
	}
	if string(shareGot) != "video:\n  frame_ms: 40\n" {
		t.Fatalf("share file changed:\n%s", shareGot)
	}
}

func TestCoreYAMLCreatesUserFile(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "nested", "config.yaml")
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configUser = user
	configSearch = []string{user}
	if err := saveAudio(false); err != nil {
		t.Fatal(err)
	}
	st := loadCore()
	if st.audio || st.shellID != "basic" || st.frameMs != 33 || st.dropFrames != 20 {
		t.Fatalf("%+v", st)
	}
}

func TestCoreListenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configUser = user
	configSearch = []string{user}
	if coreSettingsSetString(0, "not a shell") == 0 {
		t.Fatal("bad shell accepted")
	}
	listen := -1
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) == "Listen" && coreSettingsType(i) == settingString {
			listen = i
		}
	}
	if listen < 0 {
		t.Fatal("no listen row")
	}
	if coreSettingsSetString(listen, "tcp://127.0.0.1:9") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().listen != "tcp://127.0.0.1:9" {
		t.Fatalf("listen %s", loadCore().listen)
	}
	if coreSettingsGetString(listen) != "tcp://127.0.0.1:9" {
		t.Fatal(coreSettingsGetString(listen))
	}
}

func TestCoreTransportWirelessSkipsMissingGadget(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configUser = user
	configSearch = []string{user}
	row := -1
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) == "Transport" && coreSettingsType(i) == settingEnum {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no transport row")
	}
	if coreSettingsGetEnum(row) != "usb" {
		t.Fatalf("default %q", coreSettingsGetEnum(row))
	}
	if coreSettingsSetEnum(row, "wireless") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().transport != "wireless" {
		t.Fatalf("transport %s", loadCore().transport)
	}
	if coreSettingsSetEnum(row, "wpp") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().transport != "wpp" {
		t.Fatalf("transport %s", loadCore().transport)
	}
	if coreSettingsSetEnum(row, "nope") == 0 {
		t.Fatal("unknown transport accepted")
	}
	if err := os.WriteFile(user, []byte("daemon:\n  transport: cable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if loadCore().transport != "usb" {
		t.Fatalf("typo %s", loadCore().transport)
	}
}

func TestCoreDropFrames(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configUser = user
	configSearch = []string{user}
	row := -1
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) == "Shell drop frames" && coreSettingsType(i) == settingInt {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no shell drop frames row")
	}
	if coreSettingsGetInt(row) != 20 {
		t.Fatalf("default %d", coreSettingsGetInt(row))
	}
	if coreSettingsSetInt(row, 7) != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().dropFrames != 7 || coreSettingsGetInt(row) != 7 {
		t.Fatalf("stored %d", loadCore().dropFrames)
	}
	if coreSettingsSetInt(row, -1) == 0 {
		t.Fatal("negative drop frames accepted")
	}
	if err := os.WriteFile(user, []byte("shell:\n  drop_frames: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if loadCore().dropFrames != 0 {
		t.Fatalf("zero %d", loadCore().dropFrames)
	}
}

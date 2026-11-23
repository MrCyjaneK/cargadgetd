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
	if err := saveEncoder("software-fast"); err != nil {
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

func TestEncoderNamesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configSearch = []string{user}
	configUser = user
	if err := os.WriteFile(user, []byte("video:\n  encoder: \"droidmedia\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var row int
	found := false
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) != "Video encoder" {
			continue
		}
		row = i
		found = true
		if coreSettingsGetEnum(i) != "Hardware" {
			t.Fatalf("shown %q", coreSettingsGetEnum(i))
		}
		enums := coreSettingsEnums(i)
		for _, title := range []string{"Hardware", "Hardware (alt)", "software (normal)", "software (fast)"} {
			if !strings.Contains(enums, title) {
				t.Fatalf("menu %q missing %s", enums, title)
			}
		}
	}
	if !found {
		t.Fatal("video encoder row missing")
	}
	if err := saveEncoder("Hardware (alt)"); err != nil {
		t.Fatal(err)
	}
	if loadCore().encoder != "droidmedia-alt" {
		t.Fatalf("stored %s", loadCore().encoder)
	}
	if coreSettingsGetEnum(row) != "Hardware (alt)" {
		t.Fatalf("shown after save %q", coreSettingsGetEnum(row))
	}
	if err := saveEncoder("droidmedia"); err != nil {
		t.Fatal(err)
	}
	if loadCore().encoder != "droidmedia" {
		t.Fatalf("id save stored %s", loadCore().encoder)
	}
	if coreSettingsSetEnum(row, "droidmedia-alt") != 0 {
		t.Fatalf("id rejected: %s", coreSettingsError())
	}
	if loadCore().encoder != "droidmedia-alt" {
		t.Fatalf("button stored %s", loadCore().encoder)
	}
	if coreSettingsSetEnum(row, "Hardware") != 0 {
		t.Fatalf("title rejected: %s", coreSettingsError())
	}
	if loadCore().encoder != "droidmedia" {
		t.Fatalf("title stored %s", loadCore().encoder)
	}
	if coreSettingsSetEnum(row, "nope") == 0 {
		t.Fatal("unknown encoder accepted")
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
	if coreSettingsGetEnum(row) != "USB" {
		t.Fatalf("default %q", coreSettingsGetEnum(row))
	}
	enums := coreSettingsEnums(row)
	for _, title := range []string{"USB", "Wireless - developer mode", "Bluetooth"} {
		if !strings.Contains(enums, title) {
			t.Fatalf("menu %q missing %s", enums, title)
		}
	}
	if coreSettingsSetEnum(row, "wireless") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().transport != "wireless" {
		t.Fatalf("transport %s", loadCore().transport)
	}
	if coreSettingsSetEnum(row, "Bluetooth") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().transport != "wpp" {
		t.Fatalf("title stored %s", loadCore().transport)
	}
	if coreSettingsGetEnum(row) != "Bluetooth" {
		t.Fatalf("shown %q", coreSettingsGetEnum(row))
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

func TestCoreFirstRun(t *testing.T) {
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
		if coreSettingsLabel(i) == "First run" && coreSettingsType(i) == settingBool {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no first run row")
	}
	if coreSettingsGetBool(row) != 1 {
		t.Fatal("first run default is off")
	}
	if coreSettingsSetBool(row, 1) != 0 {
		t.Fatal(coreSettingsError())
	}
	if !loadCore().firstRun || coreSettingsGetBool(row) != 1 {
		t.Fatal("first run was not stored")
	}
	if coreSettingsSetBool(row, 0) != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().firstRun || coreSettingsGetBool(row) != 0 {
		t.Fatal("first run stayed on")
	}
}

func TestSaveVideoSize(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(user, []byte("video:\n  size: \"auto\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})
	configUser = user
	configSearch = []string{user}

	row := -1
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) == "Video size" && coreSettingsType(i) == settingEnum {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no video size row")
	}
	if coreSettingsEnums(row) != "auto,800x480,1280x720,1920x1080,2560x1440,3840x2160" {
		t.Fatalf("enums %q", coreSettingsEnums(row))
	}
	for _, size := range []string{"2560x1440", "3840x2160"} {
		if err := saveVideoSize(size); err != nil {
			t.Fatal(err)
		}
		if loadCore().videoSize != size {
			t.Fatalf("size %s", loadCore().videoSize)
		}
	}
	if err := saveVideoSize("640x480"); err == nil {
		t.Fatal("unknown size accepted")
	}
}

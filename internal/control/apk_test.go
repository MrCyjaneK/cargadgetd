package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListApk(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"note.txt", "b.APK", "a.apk"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rows := listApk(dir)
	if len(rows) != 3 {
		t.Fatalf("len %d", len(rows))
	}
	if !rows[0].Dir || rows[0].Name != "sub" {
		t.Fatalf("dir %+v", rows[0])
	}
	if rows[1].Name != "a.apk" || rows[1].Dir || rows[2].Name != "b.APK" {
		t.Fatalf("%+v %+v", rows[1], rows[2])
	}
	if got := listApk(filepath.Join(dir, "missing")); got != nil {
		t.Fatalf("missing dir %#v", got)
	}
}

package control

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

func LogText() string {
	b, err := os.ReadFile(LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("(no log yet: %s)", LogPath)
		}
		return fmt.Sprintf("(cannot read %s: %s)", LogPath, err.Error())
	}
	const maxTail = 64 * 1024
	if len(b) > maxTail {
		b = b[len(b)-maxTail:]
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	if len(b) == 0 {
		return "(log empty)"
	}
	return string(b)
}

func LogClear() {
	_ = os.MkdirAll(filepath.Dir(LogPath), 0o755)
	fi, err := os.Stat(LogPath)
	if err == nil && fi.Size() > 0 {
		ts := time.Now().Format("20060102-150405")
		stem := strings.TrimSuffix(LogPath, ".log")
		dest := stem + "-" + ts + ".log"
		for n := 1; n < 100; n++ {
			if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
				break
			}
			dest = fmt.Sprintf("%s-%s-%d.log", stem, ts, n)
		}
		_ = os.Rename(LogPath, dest)
	}
	if f, err := os.OpenFile(LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_ = f.Close()
	}
	pruneArchives()
}

func pruneArchives() {
	dir := filepath.Dir(LogPath)
	base := strings.TrimSuffix(filepath.Base(LogPath), ".log")
	prefix := base + "-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type aged struct {
		path string
		mod  time.Time
	}
	var old []aged
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		rest := strings.TrimSuffix(name[len(prefix):], ".log")
		if rest == "" || !unicode.IsDigit(rune(rest[0])) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		old = append(old, aged{path: filepath.Join(dir, name), mod: info.ModTime()})
	}
	sort.Slice(old, func(i, j int) bool { return old[i].mod.After(old[j].mod) })
	const keep = 30
	for i := keep; i < len(old); i++ {
		_ = os.Remove(old[i].path)
	}
}

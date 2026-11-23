package control

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type ApkRow struct {
	Name string
	Path string
	Dir  bool
}

var (
	apkMu   sync.Mutex
	apkRows []ApkRow
)

func ApkStart() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	downloads := filepath.Join(home, "Downloads")
	if st, err := os.Stat(downloads); err == nil && st.IsDir() {
		return downloads
	}
	return home
}

func ApkList(dir string) int {
	rows := listApk(dir)
	apkMu.Lock()
	apkRows = rows
	apkMu.Unlock()
	return len(rows)
}

func ApkCount() int {
	apkMu.Lock()
	defer apkMu.Unlock()
	return len(apkRows)
}

func ApkAt(i int) (ApkRow, bool) {
	apkMu.Lock()
	defer apkMu.Unlock()
	if i < 0 || i >= len(apkRows) {
		return ApkRow{}, false
	}
	return apkRows[i], true
}

func listApk(dir string) []ApkRow {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs, files []ApkRow
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		path := filepath.Join(dir, name)
		if e.IsDir() {
			dirs = append(dirs, ApkRow{Name: name, Path: path, Dir: true})
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".apk") {
			files = append(files, ApkRow{Name: name, Path: path, Dir: false})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return append(dirs, files...)
}

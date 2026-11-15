package shell

import (
	"debug/elf"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	appSymPrefix = "cargadgetd_app_"
	// appLibPrefix is the required plugin filename: libcargadgetd_app_<id>.so.
	appLibPrefix = "libcargadgetd_app_"
)

// Longest-first so cargadgetd_app_foo_resume wins over a hypothetical shorter suffix.
var appMethods = []string{"resume", "pause", "touch", "init", "draw", "name", "icon"}

func parseAppSymbol(sym string) (id, method string, ok bool) {
	if !strings.HasPrefix(sym, appSymPrefix) {
		return "", "", false
	}
	rest := sym[len(appSymPrefix):]
	// cargadgetd_app_<id>_huinput_<event> is not used for id discovery
	// (_pause would otherwise steal huinput_playpause).
	if strings.Contains(rest, "_huinput_") {
		return "", "", false
	}
	for _, m := range appMethods {
		suf := "_" + m
		if strings.HasSuffix(rest, suf) {
			id = rest[:len(rest)-len(suf)]
			if !validAppID(id) {
				return "", "", false
			}
			return id, m, true
		}
	}
	return "", "", false
}

func validAppID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if i == 0 {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
			continue
		}
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func appIDFromSymbolNames(names []string) (string, error) {
	var id string
	sawDraw := false
	for _, n := range names {
		aid, method, ok := parseAppSymbol(n)
		if !ok {
			continue
		}
		if id != "" && aid != id {
			return "", fmt.Errorf("multiple app ids %q and %q", id, aid)
		}
		id = aid
		if method == "draw" {
			sawDraw = true
		}
	}
	if id == "" || !sawDraw {
		return "", fmt.Errorf("no %s<id>_draw symbol", appSymPrefix)
	}
	return id, nil
}

// settingsOnly is an ELF libcargadgetd_app_<id>.so that exports settings_count
// and no draw. Load maps it for dlsym and does not attach it as an app.
func settingsOnly(path string) bool {
	fileID, err := appIDFromBasename(path)
	if err != nil {
		return false
	}
	f, err := elf.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	syms, err := f.DynamicSymbols()
	if err != nil {
		return false
	}
	names := make([]string, len(syms))
	for i, s := range syms {
		names[i] = s.Name
	}
	return settingsOnlyNames(fileID, names)
}

func settingsOnlyNames(fileID string, names []string) bool {
	prefix := appSymPrefix + fileID + "_"
	hasCount := false
	for _, name := range names {
		switch name {
		case prefix + "draw":
			return false
		case prefix + "settings_count":
			hasCount = true
		}
	}
	return hasCount
}

// AppIDFromFile requires libcargadgetd_app_<id>.so. ELF dynamic symbols
// (cargadgetd_app_<id>_draw) supply the id when present and must match the
// filename. A non-ELF or a library with no draw symbol uses the filename id.
func AppIDFromFile(path string) (string, error) {
	fileID, err := appIDFromBasename(path)
	if err != nil {
		return "", err
	}
	f, err := elf.Open(path)
	if err != nil {
		return fileID, nil
	}
	defer f.Close()
	syms, err := f.DynamicSymbols()
	if err != nil {
		return fileID, nil
	}
	names := make([]string, len(syms))
	for i, s := range syms {
		names[i] = s.Name
	}
	id, err := appIDFromSymbolNames(names)
	if err != nil {
		return fileID, nil
	}
	if id != fileID {
		return "", fmt.Errorf("%s: filename id %q does not match symbol id %q", path, fileID, id)
	}
	return id, nil
}

func appIDFromBasename(path string) (string, error) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, appLibPrefix) || !strings.HasSuffix(base, ".so") {
		return "", fmt.Errorf("%s: plugin must be named %s<id>.so", path, appLibPrefix)
	}
	id := strings.TrimSuffix(base[len(appLibPrefix):], ".so")
	if !validAppID(id) {
		return "", fmt.Errorf("%s: cannot derive app id from filename", path)
	}
	return id, nil
}

package cfg

import (
	"os"
	"path/filepath"
	"strings"
)

func PatchSectionKey(path, section, key, val string) error {
	if path == "" {
		return os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := ""
	if err == nil {
		text = strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	if strings.TrimSpace(text) == "" {
		return writeConfig(path, section+":\n  "+key+": "+val+"\n")
	}
	lines := strings.Split(text, "\n")
	secAt := -1
	for i, ln := range lines {
		if topKey(ln) == section {
			secAt = i
			break
		}
	}
	if secAt < 0 {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, section+":", "  "+key+": "+val, "")
		return writeConfig(path, strings.Join(lines, "\n")+"\n")
	}
	end := len(lines)
	found := -1
	for i := secAt + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if indentWidth(lines[i]) == 0 {
			end = i
			break
		}
		if childKey(trim) == key {
			found = i
		}
	}
	if found >= 0 {
		pad := leadingWS(lines[found])
		if pad == "" {
			pad = "  "
		}
		lines[found] = pad + key + ": " + val
	} else {
		line := "  " + key + ": " + val
		lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return writeConfig(path, out)
}

func topKey(line string) string {
	if indentWidth(line) != 0 {
		return ""
	}
	return childKey(strings.TrimSpace(line))
}

func childKey(trim string) string {
	if trim == "" || strings.HasPrefix(trim, "#") {
		return ""
	}
	i := strings.IndexByte(trim, ':')
	if i <= 0 {
		return ""
	}
	return trim[:i]
}

func writeConfig(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cargadgetd-*.yaml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write([]byte(body))
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		if werr != nil {
			return werr
		}
		return cerr
	}
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func indentWidth(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}

func leadingWS(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

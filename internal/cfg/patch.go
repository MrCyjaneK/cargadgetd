package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// SetKey sets section.key and writes the file back.
// A file that is not YAML is moved to config.yaml.YYYY-MM-DD.old and
// replaced with a document that contains only this key.
func SetKey(path, section, key string, value any) error {
	if path == "" || section == "" || key == "" {
		return os.ErrInvalid
	}
	m := map[string]any{}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		if uerr := yaml.Unmarshal(b, &m); uerr != nil || configOK(b) != nil {
			if _, qerr := moveAside(path); qerr != nil {
				return qerr
			}
			m = map[string]any{}
		}
	case os.IsNotExist(err):
	default:
		return err
	}
	if m == nil {
		m = map[string]any{}
	}
	sec, _ := m[section].(map[string]any)
	if sec == nil {
		sec = map[string]any{}
	}
	sec[key] = value
	m[section] = sec
	out, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return writeConfig(path, string(out))
}

// rejectBrokenUser moves a user config that is not YAML aside, writes the
// compiled defaults, and returns an error so the process can exit.
// The new file has no first_run: false, so the next start shows the wizard.
func rejectBrokenUser() error {
	p := UserPath()
	if p == "" {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := configOK(b); err != nil {
		dest, qerr := moveAside(p)
		if qerr != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		body, werr := encodeYAML(Default())
		if werr != nil {
			return werr
		}
		if werr = writeConfig(p, string(body)); werr != nil {
			return werr
		}
		return fmt.Errorf("%s: %w; moved to %s and wrote defaults", p, err, dest)
	}
	return nil
}

func configOK(b []byte) error {
	var f fileConfig
	return yaml.Unmarshal(b, &f)
}

func moveAside(path string) (string, error) {
	dest := path + "." + time.Now().Format("2006-01-02") + ".old"
	if _, err := os.Stat(dest); err == nil {
		dest = path + "." + time.Now().Format("2006-01-02-150405") + ".old"
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return dest, nil
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

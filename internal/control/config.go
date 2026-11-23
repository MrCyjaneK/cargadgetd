package control

import (
	"os"
	"strings"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
	"github.com/cargadgetd/cargadgetd/internal/rgh"
	"gopkg.in/yaml.v3"
)

const (
	shareConfig = "/usr/share/cargadgetd/config.yaml"
	etcConfig   = "/etc/cargadgetd/config.yaml"
	LogPath     = "/tmp/cargadgetd.log"
	GadgetBin   = "/usr/share/cargadgetd/bin/cargadgetd-gadget"
	MDBin       = "/usr/share/cargadgetd/bin/cargadgetd-md"
	FFSEp       = "/dev/usb-ffs/aa/ep1"
	PluginDir   = "/usr/lib/cargadgetd/plugins"
	RepoRoot    = "https://repo-oss.ftuna.eu/"
)

// configFiles, when non-nil, replaces the packaged and user YAML search.
var configFiles []string

type daemonBits struct {
	Certs    string
	FirstRun bool
}

func configPaths() []string {
	if configFiles != nil {
		return configFiles
	}
	out := []string{shareConfig, etcConfig}
	if p := cfg.UserPath(); p != "" {
		out = append(out, p)
	}
	return out
}

func readDaemon() daemonBits {
	// first_run defaults on, same as the settings row. The wizard clears it.
	out := daemonBits{Certs: rgh.ConfigDefault, FirstRun: true}
	for _, p := range configPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f struct {
			Daemon *struct {
				FirstRun *bool   `yaml:"first_run"`
				Certs    *string `yaml:"certs"`
			} `yaml:"daemon"`
		}
		if yaml.Unmarshal(b, &f) != nil || f.Daemon == nil {
			continue
		}
		if f.Daemon.Certs != nil && strings.TrimSpace(*f.Daemon.Certs) != "" {
			out.Certs = strings.TrimSpace(*f.Daemon.Certs)
		}
		if f.Daemon.FirstRun != nil {
			out.FirstRun = *f.Daemon.FirstRun
		}
	}
	return out
}

func certsBase() string {
	return rgh.ResolvePath(readDaemon().Certs)
}

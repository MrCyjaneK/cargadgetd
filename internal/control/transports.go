package control

import "github.com/cargadgetd/cargadgetd/internal/cfg"

func TransportCount() int { return len(cfg.TransportModes()) }

func TransportAt(i int) (cfg.TransportMode, bool) {
	all := cfg.TransportModes()
	if i < 0 || i >= len(all) {
		return cfg.TransportMode{}, false
	}
	return all[i], true
}

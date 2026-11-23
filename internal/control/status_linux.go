//go:build linux

package control

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func MDRunning() bool {
	return processMatches(procPattern(filepath.Base(MDBin)))
}

func AOARunning() bool {
	return processMatches("usb-aoa-launch")
}

func procPattern(name string) string {
	if name == "" {
		name = "cargadgetd-md"
	}
	return "(^|/| )" + regexp.QuoteMeta(name) + "( |$)"
}

func processMatches(pattern string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pgrep", "-f", pattern)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

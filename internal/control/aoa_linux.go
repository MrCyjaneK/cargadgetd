//go:build linux

package control

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func AoaStart() string   { return aoa("start") }
func AoaStop() string    { return aoa("stop") }
func AoaRestart() string { return aoa("restart") }

func aoa(verb string) string {
	bin := GadgetBin
	if _, err := os.Stat(bin); err != nil {
		return fmt.Sprintf("Gadget binary missing: %s", bin)
	}
	if !gadgetSetuid(bin) {
		return fmt.Sprintf("Gadget needs setuid root (RPM install). Missing setuid on %s", bin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-aoa-service", verb)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Sprintf("Handshake %s timed out", verb)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("exit %d", exitCode(err))
		}
		return fmt.Sprintf("Handshake %s failed: %s", verb, msg)
	}
	switch verb {
	case "stop":
		return "Handshake service stopped"
	case "restart":
		return "Handshake service restarted"
	default:
		return "Handshake service started"
	}
}

func gadgetSetuid(bin string) bool {
	var st syscall.Stat_t
	if err := syscall.Stat(bin, &st); err != nil {
		return false
	}
	return st.Mode&syscall.S_ISUID != 0 && st.Uid == 0
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

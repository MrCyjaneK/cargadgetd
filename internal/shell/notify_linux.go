//go:build linux

package shell

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/sessionbus"
)

func (s *Shell) WatchNotifications() {
	s.mu.Lock()
	if s.closed || s.noteStop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.noteStop = stop
	s.mu.Unlock()
	go s.watchNotes(stop)
}

func (s *Shell) watchNotes(stop <-chan struct{}) {
	if _, err := exec.LookPath("gdbus"); err != nil {
		return
	}
	seen := map[uint32]string{}
	ready := false
	tick := time.NewTicker(750 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		out, err := gdbusNotify("call", "--session",
			"--dest", "org.freedesktop.Notifications",
			"--object-path", "/org/freedesktop/Notifications",
			"--method", "org.freedesktop.Notifications.GetNotifications",
			"")
		if err != nil {
			continue
		}
		notes := parseNotificationList(out)
		if !ready {
			for _, n := range notes {
				seen[n.ID] = noteKey(n)
			}
			ready = true
			continue
		}
		var pick *rawNote
		for i := range notes {
			n := notes[i]
			if isBlankNote(n) {
				continue
			}
			k := noteKey(n)
			if prev, ok := seen[n.ID]; ok && prev == k {
				continue
			}
			seen[n.ID] = k
			if pick == nil || n.ID >= pick.ID {
				cp := n
				pick = &cp
			}
		}
		if pick == nil {
			continue
		}
		title := displayTitle(*pick)
		body := strings.TrimSpace(pick.Body)
		icon := loadNotifyIcon(pick.Icon)
		log.Printf("notify: %s — %s", title, body)
		s.showNote(title, body, icon)
	}
}

func gdbusNotify(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gdbus", args...)
	cmd.Env = sessionbus.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("gdbus timed out")
		}
		return "", err
	}
	return string(out), nil
}

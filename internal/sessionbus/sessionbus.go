// Package sessionbus finds the Sailfish user session bus.
package sessionbus

import (
	"fmt"
	"os"
	"strings"
)

// Socket returns the unix socket for the session bus. An address already
// in DBUS_SESSION_BUS_ADDRESS wins. Otherwise the usual Sailfish paths
// are tried, including the defaultuser uid 100000.
func Socket() string {
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if i := strings.Index(addr, "unix:path="); i >= 0 {
		rest := addr[i+len("unix:path="):]
		if c := strings.IndexByte(rest, ','); c >= 0 {
			rest = rest[:c]
		}
		if rest != "" {
			return rest
		}
	}
	candidates := []string{
		fmt.Sprintf("/run/user/%d/dbus/user_bus_socket", os.Getuid()),
		"/run/user/100000/dbus/user_bus_socket",
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		candidates = append([]string{xdg + "/dbus/user_bus_socket"}, candidates...)
	}
	for _, sock := range candidates {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}
	return ""
}

// Environ is the process environment with DBUS_SESSION_BUS_ADDRESS set
// when it was empty and a session socket exists.
func Environ() []string {
	env := os.Environ()
	if strings.TrimSpace(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) != "" {
		return env
	}
	if sock := Socket(); sock != "" {
		return append(env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+sock)
	}
	return env
}

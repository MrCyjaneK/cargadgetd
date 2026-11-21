//go:build linux

package wpp

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// Wait blocks until BlueZ reports that the adapter changed power or this
// peer connected or appeared. There is no timer. The read sits in the kernel
// until a signal arrives.
func Wait(mac string) error {
	c, err := net.Dial("unix", systemBusPath())
	if err != nil {
		return fmt.Errorf("wpp: system bus: %w", err)
	}
	defer c.Close()
	r := bufio.NewReader(c)
	if err := authExternal(c, r); err != nil {
		return err
	}
	dev := "dev_" + devSuffix(mac)
	if err := call(c, r, 1, "Hello", "", ""); err != nil {
		return err
	}
	rules := []string{
		"type='signal',sender='org.bluez',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'",
		"type='signal',sender='org.bluez',interface='org.freedesktop.DBus.ObjectManager',member='InterfacesAdded'",
	}
	for i, rule := range rules {
		if err := call(c, r, uint32(i+2), "AddMatch", "s", rule); err != nil {
			return err
		}
	}
	for {
		msg, err := readMsg(r)
		if err != nil {
			return fmt.Errorf("wpp: bluetooth wait: %w", err)
		}
		if bluezWake(msg, dev) {
			return nil
		}
	}
}

func systemBusPath() string {
	for _, p := range []string{"/run/dbus/system_bus_socket", "/var/run/dbus/system_bus_socket"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/run/dbus/system_bus_socket"
}

func authExternal(w io.Writer, r *bufio.Reader) error {
	uid := strconv.Itoa(os.Geteuid())
	line := "\x00AUTH EXTERNAL " + hex.EncodeToString([]byte(uid)) + "\r\n"
	if _, err := io.WriteString(w, line); err != nil {
		return fmt.Errorf("wpp: system bus: %w", err)
	}
	for {
		s, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("wpp: system bus: %w", err)
		}
		s = strings.TrimRight(s, "\r\n")
		switch {
		case strings.HasPrefix(s, "OK"):
			if _, err := io.WriteString(w, "BEGIN\r\n"); err != nil {
				return fmt.Errorf("wpp: system bus: %w", err)
			}
			return nil
		case strings.HasPrefix(s, "REJECTED"), strings.HasPrefix(s, "ERROR"):
			return fmt.Errorf("wpp: system bus: %s", s)
		}
	}
}

func call(w io.Writer, r *bufio.Reader, serial uint32, member, sig, arg string) error {
	msg := methodCall(serial, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", member, sig, arg)
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("wpp: system bus: %w", err)
	}
	// AddMatch replies can arrive after the signals they enable. The wait
	// loop frames those. Hello must complete before any other call.
	if member != "Hello" {
		return nil
	}
	for {
		got, err := readMsg(r)
		if err != nil {
			return fmt.Errorf("wpp: system bus: %w", err)
		}
		if len(got) < 2 {
			continue
		}
		switch got[1] {
		case 2:
			return nil
		case 3:
			return fmt.Errorf("wpp: system bus: hello rejected")
		}
	}
}

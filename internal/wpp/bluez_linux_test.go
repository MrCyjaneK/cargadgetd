//go:build linux

package wpp

import (
	"bufio"
	"net"
	"os"
	"testing"
	"time"
)

func TestSystemBusHello(t *testing.T) {
	bus := systemBusPath()
	if _, err := os.Stat(bus); err != nil {
		t.Skip("no system bus")
	}
	c, err := net.Dial("unix", bus)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(c)
	if err := authExternal(c, r); err != nil {
		t.Fatal(err)
	}
	if err := call(c, r, 1, "Hello", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := call(c, r, 2, "AddMatch", "s", "type='signal',sender='org.bluez',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'"); err != nil {
		t.Fatal(err)
	}
	msg, err := readMsg(r)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			t.Log("hello accepted, no bluez signal before deadline")
			return
		}
		t.Fatal(err)
	}
	if msg[1] == 3 {
		t.Fatalf("bus error: %q", msg)
	}
	t.Logf("uid handshake ok, next message type %d len %d", msg[1], len(msg))
}

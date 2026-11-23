//go:build linux

package main

import (
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestProjectionTCPUserTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	hold := make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		<-hold
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer close(hold)
	armProjectionTCP(c)
	tc := c.(*net.TCPConn)
	raw, err := tc.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var v int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		v, sockErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT)
	}); err != nil {
		t.Fatal(err)
	}
	if sockErr != nil {
		t.Fatal(sockErr)
	}
	want := int(projectionTCPUserTimeout / time.Millisecond)
	if v != want {
		t.Fatalf("TCP_USER_TIMEOUT %d, want %d", v, want)
	}
}

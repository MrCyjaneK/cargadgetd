//go:build linux

package main

import (
	"log"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

func setTCPUserTimeout(c *net.TCPConn, timeout time.Duration) {
	ms := int(timeout / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	raw, err := c.SyscallConn()
	if err != nil {
		log.Printf("projection tcp: user timeout: %v", err)
		return
	}
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, ms)
	}); err != nil {
		log.Printf("projection tcp: user timeout: %v", err)
		return
	}
	if sockErr != nil {
		log.Printf("projection tcp: user timeout: %v", sockErr)
		return
	}
	logProjectionTCP(timeout)
}

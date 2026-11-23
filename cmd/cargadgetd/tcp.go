package main

import (
	"log"
	"net"
	"time"
)

const projectionTCPUserTimeout = 8 * time.Second

func armProjectionTCP(c net.Conn) {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable:   true,
		Idle:     3 * time.Second,
		Interval: time.Second,
		Count:    3,
	})
	setTCPUserTimeout(tc, projectionTCPUserTimeout)
}

func logProjectionTCP(d time.Duration) {
	log.Printf("projection tcp: fail unacked data after %s", d)
}

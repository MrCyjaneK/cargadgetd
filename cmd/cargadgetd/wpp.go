package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
	"github.com/cargadgetd/cargadgetd/internal/wifi"
	"github.com/cargadgetd/cargadgetd/internal/wpp"
)

type wppOutcome int

const (
	wppAbsent wppOutcome = iota
	wppHandshake
	wppSession
)

// wppNext decides the next step. A finished projection dials again at once.
// A handshake that reached the head unit gets two more dials, because that
// unit is already waiting. Only a peer that was not there blocks on BlueZ.
func wppNext(out wppOutcome, misses int) (wait bool, next int) {
	switch out {
	case wppSession:
		return false, 0
	case wppHandshake:
		misses++
		if misses < 3 {
			return false, misses
		}
		return true, 0
	default:
		return true, 0
	}
}

func runWPP(serve func(io.ReadWriteCloser, string)) {
	peer := strings.TrimSpace(cfg.Get().Daemon.Peer)
	if peer == "" {
		log.Printf("wpp: daemon.peer is empty")
		return
	}
	misses := 0
	for {
		out := runWPPOnce(peer, serve)
		var wait bool
		wait, misses = wppNext(out, misses)
		if !wait {
			if out == wppSession {
				log.Printf("wpp: session ended")
			} else {
				log.Printf("wpp: handshake failed, dialing again")
			}
			continue
		}
		log.Printf("wpp: waiting for bluetooth")
		if err := wpp.Wait(peer); err != nil {
			logWPP(err)
			return
		}
	}
}

// runWPPOnce dials and runs one session. wppAbsent means the head unit was not reached.
func runWPPOnce(peer string, serve func(io.ReadWriteCloser, string)) wppOutcome {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rw, err := wpp.Dial(ctx, peer)
	if err != nil {
		logWPP(err)
		return wppAbsent
	}
	defer rw.Close()
	var gal galSession
	err = wpp.Drive(ctx, rw, func(s wpp.Setup) error {
		if err := wifi.Join(ctx, s.SSID, s.Pass); err != nil {
			return err
		}
		d := net.Dialer{Timeout: 15 * time.Second}
		c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.IP, strconv.Itoa(s.Port)))
		if err != nil {
			return err
		}
		remote := c.RemoteAddr().String()
		return gal.start(c, func() {
			serve(c, remote)
			cancel()
		})
	})
	if gal.running() {
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("wpp: bluetooth control closed, projection stays up")
		}
		gal.wait()
		return wppSession
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, errGALClosed) {
		logWPP(err)
	}
	return wppHandshake
}

func logWPP(err error) {
	msg := err.Error()
	if strings.HasPrefix(msg, "wpp:") {
		log.Print(msg)
		return
	}
	log.Printf("wpp: %v", err)
}

var errGALClosed = errors.New("wpp: bluetooth session closed")

type galSession struct {
	mu      sync.Mutex
	stopped bool
	c       io.Closer
	done    chan struct{}
}

func (g *galSession) start(c net.Conn, serve func()) error {
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		_ = c.Close()
		return errGALClosed
	}
	g.c = c
	g.done = make(chan struct{})
	go func() {
		defer close(g.done)
		serve()
	}()
	g.mu.Unlock()
	return nil
}

func (g *galSession) running() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.done != nil
}

func (g *galSession) wait() {
	g.mu.Lock()
	done := g.done
	g.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (g *galSession) stop() {
	g.mu.Lock()
	g.stopped = true
	c, done := g.c, g.done
	g.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
	if done != nil {
		<-done
	}
}

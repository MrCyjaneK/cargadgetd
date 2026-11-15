package control

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
)

// Server is a line-delimited JSON control plane.
type Server struct {
	ln net.Listener
	mu sync.Mutex
	h  map[string]Handler
}

type Handler func(req Request) Response

type Request struct {
	Cmd  string          `json:"cmd"`
	Args json.RawMessage `json:"args,omitempty"`
}

type Response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func New() *Server {
	return &Server{h: map[string]Handler{}}
}

func (s *Server) Handle(cmd string, h Handler) {
	s.mu.Lock()
	s.h[cmd] = h
	s.mu.Unlock()
}

func (s *Server) Listen(network, addr string) error {
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	s.ln = ln
	go s.accept()
	return nil
}

func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

func (s *Server) Close() error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

func (s *Server) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serve(c)
	}
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	sc := bufio.NewScanner(c)
	enc := json.NewEncoder(c)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = enc.Encode(Response{OK: false, Error: err.Error()})
			continue
		}
		s.mu.Lock()
		h := s.h[req.Cmd]
		s.mu.Unlock()
		if h == nil {
			_ = enc.Encode(Response{OK: false, Error: "unknown cmd"})
			continue
		}
		_ = enc.Encode(h(req))
	}
}

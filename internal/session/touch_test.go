package session_test

import (
	"testing"

	"github.com/cargadgetd/cargadgetd/internal/session"
)

func TestParseTouch(t *testing.T) {
	// InputEvent: field3 TouchEvent { field1 TouchLocation {x=640,y=400,pid=0}, field3 action=Press }
	// Hand-built protobuf
	loc := []byte{0x08, 0x80, 0x05, 0x10, 0x90, 0x03, 0x18, 0x00} // x=640 y=400 pid=0
	te := []byte{0x0a, byte(len(loc))}
	te = append(te, loc...)
	te = append(te, 0x18, 0x00) // action Press
	body := []byte{0x1a, byte(len(te))}
	body = append(body, te...)
	msg := append([]byte{0x80, 0x01}, body...)

	touch, ok := session.ParseTouch(msg)
	if !ok {
		t.Fatal("parse failed")
	}
	if touch.X != 640 || touch.Y != 400 {
		t.Fatalf("got %+v", touch)
	}
	if touch.Action != 0 && touch.Action != 5 {
		t.Fatal("expected press")
	}
}

func TestParseLiviTouch(t *testing.T) {
	// LIVI touch(1000, Down, x=10 y=20 id=0).
	msg := []byte{
		0x08, 0xe8, 0x07, 0x1a, 0x0c, 0x0a, 0x06, 0x08, 0x0a, 0x10, 0x14, 0x18, 0x00, 0x10, 0x00, 0x18, 0x00,
	}
	touch, ok := session.ParseTouch(msg)
	if !ok || touch.X != 10 || touch.Y != 20 || touch.Action != 0 {
		t.Fatalf("%+v %v", touch, ok)
	}
}

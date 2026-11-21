package wpp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// dbusString is how a D-Bus string sits in a message: the bytes and a NUL.
// The NUL keeps "Powered" from matching a longer name.
func dbusString(s string) []byte {
	return append([]byte(s), 0)
}

// bluezWake reports a signal worth leaving the blocked read for.
// RSSI and media position updates are ignored, so a playing headset does
// not cause another dial.
func bluezWake(msg []byte, dev string) bool {
	if bytes.Contains(msg, dbusString("Powered")) {
		return true
	}
	if dev == "" || !bytes.Contains(msg, dbusString(dev)) {
		return false
	}
	return bytes.Contains(msg, dbusString("Connected")) ||
		bytes.Contains(msg, dbusString("InterfacesAdded"))
}

type dbusMsg struct {
	buf  []byte
	base int
}

func (m *dbusMsg) align(n int) {
	for (m.base+len(m.buf))%n != 0 {
		m.buf = append(m.buf, 0)
	}
}

func (m *dbusMsg) str(s string) {
	m.align(4)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(s)))
	m.buf = append(m.buf, n[:]...)
	m.buf = append(m.buf, s...)
	m.buf = append(m.buf, 0)
}

func (m *dbusMsg) field(code byte, sig byte, s string) {
	m.align(8)
	m.buf = append(m.buf, code, 1, sig, 0)
	m.str(s)
}

func (m *dbusMsg) fieldSig(sig string) {
	m.align(8)
	m.buf = append(m.buf, 8, 1, 'g', 0)
	m.buf = append(m.buf, byte(len(sig)))
	m.buf = append(m.buf, sig...)
	m.buf = append(m.buf, 0)
}

// methodCall builds a little-endian D-Bus method call. bodySig is empty
// when the call has no arguments. bodyStr is the single string argument.
func methodCall(serial uint32, dest, path, iface, member, bodySig, bodyStr string) []byte {
	fields := dbusMsg{base: 16}
	fields.field(1, 'o', path)
	fields.field(6, 's', dest)
	fields.field(2, 's', iface)
	fields.field(3, 's', member)
	var body []byte
	if bodySig != "" {
		fields.fieldSig(bodySig)
		b := dbusMsg{}
		b.str(bodyStr)
		body = b.buf
	}
	var hdr [16]byte
	hdr[0] = 'l'
	hdr[1] = 1
	hdr[3] = 1
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(body)))
	binary.LittleEndian.PutUint32(hdr[8:12], serial)
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(len(fields.buf)))
	out := append(hdr[:], fields.buf...)
	for len(out)%8 != 0 {
		out = append(out, 0)
	}
	return append(out, body...)
}

// readMsg reads one D-Bus message. The connection stays blocked in Read
// until the daemon sends one.
func readMsg(r io.Reader) ([]byte, error) {
	var hdr [16]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	var order binary.ByteOrder
	switch hdr[0] {
	case 'l':
		order = binary.LittleEndian
	case 'B':
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("wpp: bad dbus endian %q", hdr[0])
	}
	if hdr[3] != 1 {
		return nil, fmt.Errorf("wpp: bad dbus version %d", hdr[3])
	}
	bodyLen := order.Uint32(hdr[4:8])
	fieldsLen := order.Uint32(hdr[12:16])
	total := 16 + int(fieldsLen)
	if pad := total % 8; pad != 0 {
		total += 8 - pad
	}
	total += int(bodyLen)
	if total < 16 || total > 1<<20 {
		return nil, fmt.Errorf("wpp: bad dbus message length %d", total)
	}
	msg := make([]byte, total)
	copy(msg, hdr[:])
	if _, err := io.ReadFull(r, msg[16:]); err != nil {
		return nil, err
	}
	return msg, nil
}

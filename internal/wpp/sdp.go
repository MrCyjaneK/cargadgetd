package wpp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrNoChannel = errors.New("wpp: android auto rfcomm channel not in sdp")

// Android Auto wireless RFCOMM service. LiviBluetooth registers it as "AndroidAuto".
var aaUUID = [16]byte{0x4d, 0xe1, 0x7a, 0x00, 0x52, 0xcb, 0x11, 0xe6, 0xbd, 0xf4, 0x08, 0x00, 0x20, 0x0c, 0x9a, 0x66}

var macRe = regexp.MustCompile(`(?i)[0-9a-f]{2}(?::[0-9a-f]{2}){5}`)

// parseMAC returns the address in kernel order (little-endian).
// SockaddrRFCOMM stores Addr that way. SockaddrL2 reverses Addr itself,
// so L2CAP sockets must pass l2Addr instead.
func parseMAC(s string) ([6]byte, error) {
	var wire [6]byte
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return wire, fmt.Errorf("wpp: bad mac %q", s)
	}
	var disp [6]byte
	for i, p := range parts {
		if len(p) != 2 {
			return wire, fmt.Errorf("wpp: bad mac %q", s)
		}
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return wire, fmt.Errorf("wpp: bad mac %q", s)
		}
		disp[i] = byte(v)
	}
	for i := 0; i < 6; i++ {
		wire[i] = disp[5-i]
	}
	return wire, nil
}

func macIn(s string) string {
	return macRe.FindString(s)
}

func l2Addr(wire [6]byte) [6]byte {
	var disp [6]byte
	for i := 0; i < 6; i++ {
		disp[i] = wire[5-i]
	}
	return disp
}

func devSuffix(mac string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(mac), ":", "_"))
}

func sdpSearch() []byte {
	pat := append([]byte{0x35, 0x11, 0x1c}, aaUUID[:]...)
	attr := []byte{0x35, 0x03, 0x09, 0x00, 0x04}
	param := append(append(append(pat, 0x01, 0x00), attr...), 0x00)
	pkt := make([]byte, 5+len(param))
	pkt[0] = 0x06
	binary.BigEndian.PutUint16(pkt[1:3], 1)
	binary.BigEndian.PutUint16(pkt[3:5], uint16(len(param)))
	copy(pkt[5:], param)
	return pkt
}

func channelFromSDP(buf []byte) (uint8, bool) {
	mark := []byte{0x19, 0x00, 0x03, 0x08}
	for i := 0; i+len(mark) < len(buf); i++ {
		if !bytesEqual(buf[i:i+len(mark)], mark) {
			continue
		}
		ch := buf[i+len(mark)]
		if ch >= 1 && ch <= 30 {
			return ch, true
		}
	}
	return 0, false
}

func awaitChannel(ctx context.Context, wait time.Duration, lookup func() (uint8, error)) (uint8, error) {
	misses := 0
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		ch, err := lookup()
		if !errors.Is(err, ErrNoChannel) {
			return ch, err
		}
		if misses%5 == 0 {
			log.Printf("%v - retrying", err)
		}
		misses++
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package wifi

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func wifiNoCarrier(err error) bool {
	return err != nil && strings.Contains(err.Error(), "NoCarrier")
}

func serviceReady(dump, ssid string) bool {
	ssid = strings.TrimSpace(ssid)
	if ssid == "" || dump == "" {
		return false
	}
	for _, chunk := range strings.Split(dump, "objectpath ")[1:] {
		if fieldString(chunk, "Name") != ssid {
			continue
		}
		switch fieldString(chunk, "State") {
		case "online", "ready", "association", "configuration":
			return true
		}
	}
	return false
}

func fieldString(chunk, key string) string {
	mark := "'" + key + "': <'"
	i := strings.Index(chunk, mark)
	if i < 0 {
		return ""
	}
	rest := chunk[i+len(mark):]
	j := strings.Index(rest, "'>")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func serviceSettings(ssid, pass string) string {
	parts := []string{
		"('SSID', " + gvariantString(strings.ToUpper(hex.EncodeToString([]byte(ssid)))) + ")",
		"('Security', 'psk')",
		"('Passphrase', " + gvariantString(pass) + ")",
		"('Hidden', 'true')",
		"('AutoConnect', 'true')",
	}
	if ssid != "" && utf8.ValidString(ssid) && !strings.ContainsRune(ssid, 0) {
		parts = append([]string{"('Name', " + gvariantString(ssid) + ")"}, parts...)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gvariantString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func writeCred(ssid, pass string) []byte {
	var b []byte
	put := func(s string) {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(s)))
		b = append(b, n[:]...)
		b = append(b, s...)
	}
	put(ssid)
	put(pass)
	return b
}

func ReadCred(r io.Reader) (ssid, pass string, err error) {
	one := func() (string, error) {
		var n uint32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return "", err
		}
		if n > 256 {
			return "", fmt.Errorf("wifi: credential too long")
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	}
	ssid, err = one()
	if err != nil {
		return "", "", err
	}
	pass, err = one()
	return ssid, pass, err
}

func objectPath(out string) (string, bool) {
	for _, mark := range []string{"objectpath '", `objectpath "`} {
		i := strings.Index(out, mark)
		if i < 0 {
			continue
		}
		rest := out[i+len(mark):]
		end := strings.IndexAny(rest, `'"`)
		if end <= 0 {
			continue
		}
		return rest[:end], true
	}
	return "", false
}

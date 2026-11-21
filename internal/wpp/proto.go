package wpp

import (
	"encoding/binary"
	"errors"
)

var errProto = errors.New("wpp: bad protobuf")

const (
	idStart    = 1
	idInfoReq  = 2
	idInfo     = 3
	idVersion  = 4
	idVersionR = 5
	idConnect  = 6
	idPing     = 8
	idPong     = 9
	idReject   = 10
)

const (
	statusOK          = 0
	statusCredentials = -3
	statusNoNetwork   = -11
	securityWPA2      = 8
)

type protoField struct {
	num    int
	varint bool
	u      uint64
	raw    []byte
}

func parseProto(b []byte) ([]protoField, error) {
	var out []protoField
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, errProto
		}
		b = b[n:]
		num := int(key >> 3)
		switch key & 7 {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, errProto
			}
			b = b[n:]
			out = append(out, protoField{num: num, varint: true, u: v})
		case 1:
			if len(b) < 8 {
				return nil, errProto
			}
			b = b[8:]
		case 2:
			ln, n := binary.Uvarint(b)
			if n <= 0 || uint64(len(b[n:])) < ln {
				return nil, errProto
			}
			b = b[n:]
			out = append(out, protoField{num: num, raw: append([]byte(nil), b[:ln]...)})
			b = b[ln:]
		case 5:
			if len(b) < 4 {
				return nil, errProto
			}
			b = b[4:]
		default:
			return nil, errProto
		}
	}
	return out, nil
}

func appendKeyVarint(b []byte, field int, v uint64) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3)
	return binary.AppendUvarint(b, v)
}

func appendKeyBytes(b []byte, field int, s string) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|2)
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

func versionBody(req []byte) []byte {
	maj, min := uint64(6), uint64(0)
	if fields, err := parseProto(req); err == nil {
		for _, f := range fields {
			if !f.varint {
				continue
			}
			switch f.num {
			case 1:
				maj = f.u
			case 2:
				min = f.u
			}
		}
	}
	var b []byte
	b = appendKeyVarint(b, 1, maj)
	b = appendKeyVarint(b, 2, min)
	b = appendKeyVarint(b, 4, statusOK)
	return b
}

func statusBody(code int32) []byte {
	return appendKeyVarint(nil, 1, uint64(int64(code)))
}

func statusCode(body []byte) (int32, bool) {
	fields, err := parseProto(body)
	if err != nil {
		return 0, false
	}
	for _, f := range fields {
		if f.num == 1 && f.varint {
			return int32(f.u), true
		}
	}
	return 0, false
}

func parseStart(body []byte) (ip string, port int, err error) {
	fields, err := parseProto(body)
	if err != nil {
		return "", 0, err
	}
	for _, f := range fields {
		switch f.num {
		case 1:
			ip = string(f.raw)
		case 2:
			if f.varint {
				port = int(f.u)
			}
		}
	}
	if ip == "" || port == 0 {
		return "", 0, errProto
	}
	return ip, port, nil
}

func parseInfo(body []byte) (ssid, pass, bssid string, security int, err error) {
	fields, err := parseProto(body)
	if err != nil {
		return "", "", "", 0, err
	}
	var gotSec bool
	for _, f := range fields {
		switch f.num {
		case 1:
			ssid = string(f.raw)
		case 2:
			pass = string(f.raw)
		case 3:
			bssid = string(f.raw)
		case 4:
			if f.varint {
				security = int(f.u)
				gotSec = true
			}
		}
	}
	if ssid == "" || !gotSec {
		return "", "", "", 0, errProto
	}
	return ssid, pass, bssid, security, nil
}

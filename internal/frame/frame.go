package frame

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	FlagFirst     = 0x01
	FlagLast      = 0x02
	FlagBulk      = FlagFirst | FlagLast
	FlagSpecific  = 0x04
	FlagEncrypted = 0x08
)

// MaxPlainChunk is the max plaintext bytes per on-wire AA frame (TCP path).
// Must stay ≤ TLS max record plaintext (16384) so each AA frame is one TLS record.
const MaxPlainChunk = 1600

// MaxPlainChunkUSB is the USB bulk plaintext budget per AA frame / TLS record (≤16384).
const MaxPlainChunkUSB = 12000

// Frame is one on-wire Android Auto frame (cleartext header).
type Frame struct {
	Channel uint8
	Flags   uint8
	// TotalLen is set when FlagFirst is set without FlagLast (fragment start).
	// For encrypted traffic this is the total *plaintext* length (AACS/DHU).
	TotalLen uint32
	Payload  []byte
}

var ErrShort = errors.New("frame: short read")

// EncryptFunc turns one plaintext chunk into ciphertext for a single frame.
type EncryptFunc func(plain []byte) ([]byte, error)

func Read(r io.Reader) (Frame, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Frame{}, err
	}
	f := Frame{
		Channel: hdr[0],
		Flags:   hdr[1],
	}
	plen := binary.BigEndian.Uint16(hdr[2:4])
	if f.Flags&FlagFirst != 0 && f.Flags&FlagLast == 0 {
		var tl [4]byte
		if _, err := io.ReadFull(r, tl[:]); err != nil {
			return Frame{}, err
		}
		f.TotalLen = binary.BigEndian.Uint32(tl[:])
	}
	f.Payload = make([]byte, plen)
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// Write sends one logical message. For plain payloads it may fragment.
// Prefer WriteEncrypted for encrypted channels (AACS: encrypt per chunk).
func Write(w io.Writer, f Frame) error {
	payload := f.Payload
	flags := f.Flags

	if len(payload) <= MaxPlainChunk {
		return writeOne(w, f.Channel, flags|FlagBulk, 0, payload)
	}

	total := uint32(len(payload))
	first := payload[:MaxPlainChunk]
	if err := writeOne(w, f.Channel, (flags&^FlagBulk)|FlagFirst, total, first); err != nil {
		return err
	}
	rest := payload[MaxPlainChunk:]
	for len(rest) > MaxPlainChunk {
		chunk := rest[:MaxPlainChunk]
		rest = rest[MaxPlainChunk:]
		if err := writeOne(w, f.Channel, flags&^FlagBulk, 0, chunk); err != nil {
			return err
		}
	}
	return writeOne(w, f.Channel, (flags&^FlagBulk)|FlagLast, 0, rest)
}

// WriteEncrypted fragments plaintext, encrypts each chunk (complete TLS record
// per frame), and sets First-frame total to the plaintext length - matching AACS.
func WriteEncrypted(w io.Writer, channel, flags uint8, plain []byte, enc EncryptFunc) error {
	return WriteEncryptedYield(w, channel, flags, plain, enc, nil, MaxPlainChunk)
}

// WriteEncryptedYield is WriteEncrypted plus an optional hook between on-wire
// fragments (for flushing control/ping). maxChunk caps plaintext per AA frame
// (AACS USB uses 2000; keep ≤16384 so each AA frame is one TLS record).
func WriteEncryptedYield(w io.Writer, channel, flags uint8, plain []byte, enc EncryptFunc, between func(), maxChunk int) error {
	flags = (flags &^ FlagBulk) | FlagEncrypted
	if len(plain) == 0 {
		return nil
	}
	if maxChunk <= 0 {
		maxChunk = MaxPlainChunk
	}
	// AA payload length is uint16; leave room for TLS record overhead.
	if maxChunk > 60000 {
		maxChunk = 60000
	}
	if len(plain) <= maxChunk {
		cipher, err := enc(plain)
		if err != nil {
			return err
		}
		return writeOne(w, channel, flags|FlagBulk, 0, cipher)
	}

	total := uint32(len(plain))
	offset := 0
	first := true
	for offset < len(plain) {
		if between != nil {
			between()
		}
		end := offset + maxChunk
		if end > len(plain) {
			end = len(plain)
		}
		chunk := plain[offset:end]
		offset = end
		var fflags uint8
		var totalField uint32
		switch {
		case first && offset == len(plain):
			fflags = flags | FlagBulk
		case first:
			fflags = flags | FlagFirst
			totalField = total
		case offset == len(plain):
			fflags = flags | FlagLast
		default:
			fflags = flags
		}
		first = false
		cipher, err := enc(chunk)
		if err != nil {
			return err
		}
		if err := writeOne(w, channel, fflags, totalField, cipher); err != nil {
			return err
		}
	}
	return nil
}

func appendFrame(buf []byte, channel, flags uint8, total uint32, payload []byte) []byte {
	buf = append(buf, channel, flags)
	buf = PutU16(buf, uint16(len(payload)))
	if flags&FlagFirst != 0 && flags&FlagLast == 0 {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], total)
		buf = append(buf, tmp[:]...)
	}
	return append(buf, payload...)
}

func writeOne(w io.Writer, channel, flags uint8, total uint32, payload []byte) error {
	// Single write - some HU stacks (DHU) mishandle split TCP segments on the first frame.
	_, err := w.Write(appendFrame(nil, channel, flags, total, payload))
	return err
}

func PutU16(b []byte, v uint16) []byte {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], v)
	return append(b, tmp[:]...)
}

func PutU64(b []byte, v uint64) []byte {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], v)
	return append(b, tmp[:]...)
}

func GetU16(b []byte) uint16 {
	return binary.BigEndian.Uint16(b)
}

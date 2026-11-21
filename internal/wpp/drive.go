package wpp

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

type Setup struct {
	SSID, Pass, BSSID string
	Security          int
	IP                string
	Port              int
}

func Drive(ctx context.Context, rw io.ReadWriter, join func(Setup) error) error {
	frames := make(chan frame, 8)
	readErr := make(chan error, 1)
	go func() {
		for {
			f, err := readFrame(rw)
			if err != nil {
				readErr <- err
				return
			}
			frames <- f
		}
	}()

	var setup Setup
	var haveStart bool
	joinErr := make(chan error, 1)
	var joined bool
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErr:
			return err
		case err := <-joinErr:
			joinErr = nil
			if err != nil {
				_ = writeFrame(rw, idConnect, statusBody(statusNoNetwork))
				return err
			}
			if err := writeFrame(rw, idConnect, statusBody(statusOK)); err != nil {
				return err
			}
		case f := <-frames:
			switch f.id {
			case idPing:
				if err := writeFrame(rw, idPong, f.body); err != nil {
					return err
				}
			case idVersion:
				if err := writeFrame(rw, idVersionR, versionBody(f.body)); err != nil {
					return err
				}
			case idStart:
				ip, port, err := parseStart(f.body)
				if err != nil {
					return err
				}
				setup.IP, setup.Port = ip, port
				haveStart = true
				if err := writeFrame(rw, idInfoReq, nil); err != nil {
					return err
				}
			case idInfo:
				if !haveStart {
					return fmt.Errorf("wpp: info before start")
				}
				ssid, pass, bssid, sec, err := parseInfo(f.body)
				if err != nil {
					return err
				}
				setup.SSID, setup.Pass, setup.BSSID, setup.Security = ssid, pass, bssid, sec
				if sec != securityWPA2 {
					_ = writeFrame(rw, idConnect, statusBody(statusCredentials))
					return fmt.Errorf("wpp: security mode %d", sec)
				}
				if joined {
					continue
				}
				joined = true
				s := setup
				go func() { joinErr <- join(s) }()
			case idReject:
				return fmt.Errorf("wpp: connection rejected")
			}
		}
	}
}

type frame struct {
	id   uint16
	body []byte
}

func readFrame(r io.Reader) (frame, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return frame{}, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:2]))
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	return frame{id: binary.BigEndian.Uint16(hdr[2:]), body: body}, nil
}

func writeFrame(w io.Writer, id uint16, body []byte) error {
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(body)))
	binary.BigEndian.PutUint16(buf[2:], id)
	copy(buf[4:], body)
	_, err := w.Write(buf)
	return err
}

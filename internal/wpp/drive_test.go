package wpp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// LIVI's version request for channel 36: major 6, minor 0, 5180 MHz.
var liviVersion = []byte{0x00, 0x07, 0x00, 0x04, 0x08, 0x06, 0x10, 0x00, 0x20, 0xbc, 0x28}

func TestDriveLIVITracePingWhileJoinBlocked(t *testing.T) {
	hu, phone := net.Pipe()
	defer hu.Close()
	defer phone.Close()
	_ = hu.SetDeadline(time.Now().Add(3 * time.Second))

	release := make(chan struct{})
	entered := make(chan struct{})
	errCh := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		errCh <- Drive(ctx, phone, func(s Setup) error {
			if s.SSID != "LIVI" || s.Pass != "secret" || s.BSSID != "aa:bb:cc:dd:ee:ff" || s.Security != 8 || s.IP != "10.0.0.1" || s.Port != 5277 {
				return errors.New("bad setup")
			}
			close(entered)
			<-release
			return nil
		})
	}()

	if _, err := hu.Write(liviVersion); err != nil {
		t.Fatal(err)
	}
	ver := mustFrame(t, hu)
	if ver.id != idVersionR || !bytes.Contains(ver.body, []byte{0x08, 0x06}) || !bytes.Contains(ver.body, []byte{0x20, 0x00}) {
		t.Fatalf("version response id=%d body=%x", ver.id, ver.body)
	}

	start := appendKeyBytes(nil, 1, "10.0.0.1")
	start = appendKeyVarint(start, 2, 5277)
	if err := writeFrame(hu, idStart, start); err != nil {
		t.Fatal(err)
	}
	infoReq := mustFrame(t, hu)
	if infoReq.id != idInfoReq || len(infoReq.body) != 0 {
		t.Fatalf("info request id=%d body=%x", infoReq.id, infoReq.body)
	}

	info := appendKeyBytes(nil, 1, "LIVI")
	info = appendKeyBytes(info, 2, "secret")
	info = appendKeyBytes(info, 3, "aa:bb:cc:dd:ee:ff")
	info = appendKeyVarint(info, 4, securityWPA2)
	info = appendKeyVarint(info, 5, 0)
	if err := writeFrame(hu, idInfo, info); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("join was not called")
	}

	if err := writeFrame(hu, idPing, []byte{0xaa}); err != nil {
		t.Fatal(err)
	}
	pong := mustFrame(t, hu)
	if pong.id != idPong || !bytes.Equal(pong.body, []byte{0xaa}) {
		t.Fatalf("pong id=%d body=%x", pong.id, pong.body)
	}
	close(release)

	st := mustFrame(t, hu)
	code, ok := statusCode(st.body)
	if st.id != idConnect || !ok || code != statusOK {
		t.Fatalf("connect id=%d ok=%v code=%d body=%x", st.id, ok, code, st.body)
	}
	_ = hu.Close()
	if err := <-errCh; err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDriveStartFirstSkipsVersion(t *testing.T) {
	hu, phone := net.Pipe()
	defer hu.Close()
	defer phone.Close()
	_ = hu.SetDeadline(time.Now().Add(3 * time.Second))

	errCh := make(chan error, 1)
	go func() {
		errCh <- Drive(context.Background(), phone, func(Setup) error { return nil })
	}()

	start := appendKeyBytes(nil, 1, "192.168.43.1")
	start = appendKeyVarint(start, 2, 5277)
	if err := writeFrame(hu, idStart, start); err != nil {
		t.Fatal(err)
	}
	infoReq := mustFrame(t, hu)
	if infoReq.id != idInfoReq {
		t.Fatalf("first reply id=%d, want info request", infoReq.id)
	}
	info := appendKeyBytes(nil, 1, "old")
	info = appendKeyBytes(info, 2, "pw")
	info = appendKeyVarint(info, 4, securityWPA2)
	if err := writeFrame(hu, idInfo, info); err != nil {
		t.Fatal(err)
	}
	st := mustFrame(t, hu)
	code, ok := statusCode(st.body)
	if st.id != idConnect || !ok || code != statusOK {
		t.Fatalf("connect id=%d code=%d", st.id, code)
	}
	_ = hu.Close()
	<-errCh
}

func TestDriveRejectsOtherSecurity(t *testing.T) {
	hu, phone := net.Pipe()
	defer hu.Close()
	defer phone.Close()
	_ = hu.SetDeadline(time.Now().Add(3 * time.Second))

	called := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Drive(context.Background(), phone, func(Setup) error {
			called <- struct{}{}
			return nil
		})
	}()

	if _, err := hu.Write(liviVersion); err != nil {
		t.Fatal(err)
	}
	if f := mustFrame(t, hu); f.id != idVersionR {
		t.Fatalf("version id=%d", f.id)
	}
	start := appendKeyBytes(nil, 1, "10.0.0.1")
	start = appendKeyVarint(start, 2, 5277)
	if err := writeFrame(hu, idStart, start); err != nil {
		t.Fatal(err)
	}
	if f := mustFrame(t, hu); f.id != idInfoReq {
		t.Fatalf("info id=%d", f.id)
	}
	info := appendKeyBytes(nil, 1, "LIVI")
	info = appendKeyBytes(info, 2, "secret")
	info = appendKeyVarint(info, 4, 40)
	if err := writeFrame(hu, idInfo, info); err != nil {
		t.Fatal(err)
	}
	st := mustFrame(t, hu)
	code, ok := statusCode(st.body)
	if st.id != idConnect || !ok || code >= 0 {
		t.Fatalf("connect id=%d ok=%v code=%d", st.id, ok, code)
	}
	select {
	case <-called:
		t.Fatal("join called for security 40")
	default:
	}
	if err := <-errCh; err == nil {
		t.Fatal("expected security error")
	}
}

func TestStatusRoundTrip(t *testing.T) {
	for _, code := range []int32{0, -3, -11} {
		got, ok := statusCode(statusBody(code))
		if !ok || got != code {
			t.Fatalf("status %d -> %d ok=%v", code, got, ok)
		}
	}
}

func mustFrame(t *testing.T, r io.Reader) frame {
	t.Helper()
	f, err := readFrame(r)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

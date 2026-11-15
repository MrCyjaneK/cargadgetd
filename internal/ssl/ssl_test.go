package ssl_test

import (
	"path/filepath"
	"testing"

	aassl "github.com/cargadgetd/cargadgetd/internal/ssl"
	"github.com/cargadgetd/cargadgetd/internal/testpem"
)

func TestHandshake(t *testing.T) {
	dir := testpem.Dir(t)
	srv, err := aassl.NewServer(filepath.Join(dir, "android_auto.crt"), filepath.Join(dir, "android_auto.key"))
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	cli, err := aassl.NewClient(filepath.Join(dir, "headunit.crt"), filepath.Join(dir, "headunit.key"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	handshakeAndPing(t, srv, cli)
}

func TestHandshakePEM(t *testing.T) {
	md := testpem.Mint(t)
	hu := testpem.Mint(t)
	srv, err := aassl.NewServerPEM(md.Cert, md.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	cli, err := aassl.NewClientPEM(hu.Cert, hu.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	handshakeAndPing(t, srv, cli)
}

func handshakeAndPing(t *testing.T, srv, cli *aassl.Session) {
	t.Helper()
	var (
		toSrv []byte
		toCli []byte
		err   error
	)
	srvDone, cliDone := false, false
	toCli, cliDone, err = cli.FeedHandshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30 && !(srvDone && cliDone); i++ {
		toSrv, srvDone, err = srv.FeedHandshake(toCli)
		if err != nil {
			t.Fatalf("server: %v", err)
		}
		toCli, cliDone, err = cli.FeedHandshake(toSrv)
		if err != nil {
			t.Fatalf("client: %v", err)
		}
	}
	if !srvDone || !cliDone {
		t.Fatalf("handshake incomplete srv=%v cli=%v", srvDone, cliDone)
	}

	ct, err := srv.Encrypt([]byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := cli.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "ping" {
		t.Fatalf("got %q", pt)
	}
	ct2, err := cli.Encrypt([]byte("pong"))
	if err != nil {
		t.Fatal(err)
	}
	pt2, err := srv.Decrypt(ct2)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt2) != "pong" {
		t.Fatalf("got %q", pt2)
	}
}

func TestEncryptLarge(t *testing.T) {
	dir := testpem.Dir(t)
	srv, err := aassl.NewServer(filepath.Join(dir, "android_auto.crt"), filepath.Join(dir, "android_auto.key"))
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	cli, err := aassl.NewClient(filepath.Join(dir, "headunit.crt"), filepath.Join(dir, "headunit.key"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	handshakeAndPing(t, srv, cli)

	// One TLS record max — plain ≤16384; USB path uses MaxPlainChunkUSB.
	for _, sz := range []int{1600, 2000, 14000, 16384} {
		plain := make([]byte, sz)
		for i := range plain {
			plain[i] = byte(i)
		}
		ct, err := srv.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt %d: %v", sz, err)
		}
		pt, err := cli.Decrypt(ct)
		if err != nil {
			t.Fatalf("decrypt %d: %v", sz, err)
		}
		if len(pt) != sz {
			t.Fatalf("sz=%d got pt=%d ct=%d", sz, len(pt), len(ct))
		}
	}
}

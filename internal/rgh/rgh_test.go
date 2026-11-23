package rgh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolvePath(t *testing.T) {
	want := ResolvePath("")
	if got := ResolvePath("  "); got != want {
		t.Fatalf("blank: %q", got)
	}
	if !strings.HasSuffix(want, "/.local/share/cargadgetd/certs") {
		t.Fatalf("default: %q", want)
	}
	if got := ResolvePath("/tmp/custom"); got != "/tmp/custom" {
		t.Fatalf("custom: %q", got)
	}
	if Canonical("") != ConfigDefault {
		t.Fatalf("canonical: %q", Canonical(""))
	}
}

func TestOpenMissing(t *testing.T) {
	_, err := Open("/no/such/certs")
	if err == nil {
		t.Fatal("expected error")
	}
	le, ok := err.(*LoadError)
	if !ok || !le.Missing {
		t.Fatalf("want missing LoadError, got %T %v", err, err)
	}
	if err.Error() != "android auto certificate is missing (/no/such/certs)" {
		t.Fatalf("msg: %q", err.Error())
	}
}

func TestOpenEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	le, ok := err.(*LoadError)
	if !ok || !le.Missing {
		t.Fatalf("want missing LoadError, got %v", err)
	}
}

func TestOpenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("msg: %v", err)
	}
}

func TestOpenPair(t *testing.T) {
	dir := t.TempDir()
	cert, key := mint(t)
	if err := os.WriteFile(filepath.Join(dir, "android_auto.crt"), cert, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "android_auto.key"), key, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Probe, "key: matches leaf") || m.ExpiresIn <= 0 {
		t.Fatalf("probe %q expires %d", m.Probe, m.ExpiresIn)
	}

	other, _ := mint(t)
	if err := os.WriteFile(filepath.Join(dir, "android_auto.crt"), other, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatch: %v", err)
	}
}

func mint(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "rgh-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kder, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kder})
}

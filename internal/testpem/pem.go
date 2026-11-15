package testpem

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Pair is a self-signed certificate and its PKCS#8 private key.
type Pair struct {
	Cert []byte
	Key  []byte
}

func Mint(tb testing.TB) Pair {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		tb.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "cargadgetd-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		tb.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		tb.Fatal(err)
	}
	return Pair{
		Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		Key:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}),
	}
}

func Dir(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	write(tb, dir, "android_auto", Mint(tb))
	write(tb, dir, "headunit", Mint(tb))
	return dir
}

func write(tb testing.TB, dir, name string, p Pair) {
	tb.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".crt"), p.Cert, 0o600); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".key"), p.Key, 0o600); err != nil {
		tb.Fatal(err)
	}
}

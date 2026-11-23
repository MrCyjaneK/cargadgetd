// Package rgh loads GAL MD TLS material from a certificate directory.
package rgh

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ConfigDefault = "~/.local/share/cargadgetd/certs"

type Material struct {
	Path      string
	Cert      []byte
	Key       []byte
	CA        []byte
	Probe     string
	ExpiresIn int64
}

func Canonical(certs string) string {
	s := strings.TrimSpace(certs)
	if s == "" {
		return ConfigDefault
	}
	return s
}

func ResolvePath(certs string) string {
	return expandUser(Canonical(certs))
}

func expandUser(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}

func missingErr(path string) error {
	return &LoadError{Path: path, Missing: true}
}

type LoadError struct {
	Path    string
	Missing bool
	Err     error
}

func (e *LoadError) Error() string {
	if e == nil {
		return "android auto certificate is missing"
	}
	if e.Missing {
		if e.Path != "" {
			return "android auto certificate is missing (" + e.Path + ")"
		}
		return "android auto certificate is missing"
	}
	if e.Err != nil {
		if e.Path != "" {
			return "android auto certificate failed to load (" + e.Path + "): " + e.Err.Error()
		}
		return "android auto certificate failed to load: " + e.Err.Error()
	}
	return "android auto certificate failed to load"
}

func (e *LoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Open(path string) (*Material, error) {
	path = ResolvePath(path)
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, missingErr(path)
		}
		return nil, &LoadError{Path: path, Err: err}
	}
	if !st.IsDir() {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("need a certificate directory")}
	}
	cert, err := os.ReadFile(filepath.Join(path, "android_auto.crt"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, missingErr(path)
		}
		return nil, &LoadError{Path: path, Err: err}
	}
	key, err := os.ReadFile(filepath.Join(path, "android_auto.key"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, missingErr(path)
		}
		return nil, &LoadError{Path: path, Err: err}
	}
	if len(cert) == 0 || len(key) == 0 {
		return nil, missingErr(path)
	}
	probe, exp, err := describe(cert, key)
	if err != nil {
		return nil, &LoadError{Path: path, Err: err}
	}
	m := &Material{Path: path, Cert: cert, Key: key, Probe: probe, ExpiresIn: exp}
	if ca, err := os.ReadFile(filepath.Join(path, "ca.crt")); err == nil {
		m.CA = ca
	}
	return m, nil
}

func describe(certPEM, keyPEM []byte) (string, int64, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", 0, errors.New("android_auto.crt is not a PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", 0, fmt.Errorf("android_auto.crt: %w", err)
	}
	kblock, _ := pem.Decode(keyPEM)
	if kblock == nil {
		return "", 0, errors.New("android_auto.key is not PEM")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(kblock.Bytes)
	if err != nil {
		return "", 0, fmt.Errorf("android_auto.key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return "", 0, fmt.Errorf("android_auto.key is %T, not RSA", keyAny)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return "", 0, fmt.Errorf("leaf public key is %T, not RSA", cert.PublicKey)
	}
	if pub.N.Cmp(key.N) != 0 {
		return "", 0, errors.New("private key does not match the leaf certificate")
	}
	exp := int64(time.Until(cert.NotAfter).Seconds())
	if exp < 0 {
		exp = 0
	}
	text := fmt.Sprintf("subject: %s\nnotAfter: %s\nkey: matches leaf\n",
		cert.Subject.String(), cert.NotAfter.UTC().Format(time.RFC3339))
	return text, exp, nil
}

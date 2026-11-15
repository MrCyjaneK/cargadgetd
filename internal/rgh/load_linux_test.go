//go:build linux && cgo

package rgh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeSrc = `
#include <stdint.h>
#include <stddef.h>

static uint8_t crt[] = "-----BEGIN CERTIFICATE-----\nMIIBfake\n-----END CERTIFICATE-----\n";
static uint8_t key[] = "-----BEGIN PRIVATE KEY-----\nMIIBfake\n-----END PRIVATE KEY-----\n";
static uint8_t ca[] = "-----BEGIN CERTIFICATE-----\nCAfake\n-----END CERTIFICATE-----\n";
static char probe[] = "android_auto.crt: fake\nandroid_auto.key: fake\nca.crt: fake\n";

uint8_t *reversegearhead_android_auto_crt(size_t *n) {
	if (n) *n = sizeof(crt) - 1;
	return crt;
}
uint8_t *reversegearhead_android_auto_key(size_t *n) {
	if (n) *n = sizeof(key) - 1;
	return key;
}
uint8_t *reversegearhead_ca_crt(size_t *n) {
	if (n) *n = sizeof(ca) - 1;
	return ca;
}
char *reversegearhead_probe(void) { return probe; }
long long reversegearhead_expires_in(void) { return 12345; }
`

func buildFake(t *testing.T, src, name string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	dir := t.TempDir()
	cpath := filepath.Join(dir, "fake.c")
	if err := os.WriteFile(cpath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	so := filepath.Join(dir, name)
	out, err := exec.Command("gcc", "-shared", "-fPIC", "-o", so, cpath).CombinedOutput()
	if err != nil {
		t.Fatalf("gcc: %v (%s)", err, out)
	}
	return so
}

func TestOpenFake(t *testing.T) {
	so := buildFake(t, fakeSrc, "reversegearhead.so")
	m, err := Open(so)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m.Cert), "BEGIN CERTIFICATE") {
		t.Fatalf("cert: %q", m.Cert)
	}
	if !strings.Contains(string(m.Key), "BEGIN PRIVATE KEY") {
		t.Fatalf("key: %q", m.Key)
	}
	if !strings.Contains(string(m.CA), "CAfake") {
		t.Fatalf("ca: %q", m.CA)
	}
	if m.Probe != "android_auto.crt: fake\nandroid_auto.key: fake\nca.crt: fake\n" {
		t.Fatalf("probe: %q", m.Probe)
	}
	if m.ExpiresIn != 12345 {
		t.Fatalf("expires: %d", m.ExpiresIn)
	}
}

func TestOpenMissingSymbols(t *testing.T) {
	src := `int dummy(void) { return 1; }`
	so := buildFake(t, src, "empty.so")
	_, err := Open(so)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*LoadError); !ok {
		t.Fatalf("want LoadError, got %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "failed to load") {
		t.Fatalf("msg: %q", err.Error())
	}
}

func TestOpenDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("msg: %q", err.Error())
	}
}

func TestOpenMissing(t *testing.T) {
	_, err := Open("/no/such/reversegearhead.so")
	if err == nil {
		t.Fatal("expected error")
	}
	le, ok := err.(*LoadError)
	if !ok || !le.Missing {
		t.Fatalf("want missing LoadError, got %T %v", err, err)
	}
	if err.Error() != "reversegearhead.so is missing (/no/such/reversegearhead.so)" {
		t.Fatalf("msg: %q", err.Error())
	}
}

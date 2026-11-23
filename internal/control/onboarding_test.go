package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDaemonMerge(t *testing.T) {
	dir := t.TempDir()
	early := filepath.Join(dir, "early.yaml")
	late := filepath.Join(dir, "late.yaml")
	if err := os.WriteFile(early, []byte("daemon:\n  certs: \"/first\"\n  first_run: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(late, []byte("daemon:\n  certs: \"/second\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configFiles
	configFiles = []string{early, late}
	t.Cleanup(func() { configFiles = prev })
	got := readDaemon()
	if got.Certs != "/second" || got.FirstRun {
		t.Fatalf("%+v", got)
	}
}

func TestOnboardingFirstRun(t *testing.T) {
	dir := t.TempDir()
	prev := configFiles
	configFiles = []string{}
	t.Cleanup(func() { configFiles = prev })
	if OnboardingDone() {
		t.Fatal("unset first_run still shows the wizard")
	}

	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("daemon:\n  first_run: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFiles = []string{yamlPath}
	if OnboardingDone() {
		t.Fatal("first_run shows the wizard")
	}
	if err := os.WriteFile(yamlPath, []byte("daemon:\n  first_run: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !OnboardingDone() {
		t.Fatal("clearing first_run finishes onboarding")
	}
}

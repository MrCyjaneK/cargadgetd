package usbcable

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOnlineMissingSysfs(t *testing.T) {
	oldOn, oldEx := onlineFiles, extconFiles
	t.Cleanup(func() { onlineFiles, extconFiles = oldOn, oldEx })
	onlineFiles = []string{filepath.Join(t.TempDir(), "nope")}
	extconFiles = []string{filepath.Join(t.TempDir(), "nope")}
	if !Online() {
		t.Fatal("missing sysfs must assume connected")
	}
}

func TestOnlineFile(t *testing.T) {
	oldOn, oldEx := onlineFiles, extconFiles
	t.Cleanup(func() { onlineFiles, extconFiles = oldOn, oldEx })
	dir := t.TempDir()
	p := filepath.Join(dir, "online")
	onlineFiles = []string{p}
	extconFiles = nil

	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !Online() {
		t.Fatal("want online")
	}
	on, src, raw := Probe()
	if !on || src != p || raw != "1" {
		t.Fatalf("Probe: on=%v src=%s raw=%q", on, src, raw)
	}
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if Online() {
		t.Fatal("want offline")
	}
}

func TestOnlineExtcon(t *testing.T) {
	oldOn, oldEx := onlineFiles, extconFiles
	t.Cleanup(func() { onlineFiles, extconFiles = oldOn, oldEx })
	dir := t.TempDir()
	p := filepath.Join(dir, "state")
	onlineFiles = []string{filepath.Join(dir, "missing")}
	extconFiles = []string{p}

	if err := os.WriteFile(p, []byte("USB=1\nUSB-HOST=0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !Online() {
		t.Fatal("want USB=1")
	}
	if err := os.WriteFile(p, []byte("USB=0\nUSB-HOST=0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if Online() {
		t.Fatal("want USB=0")
	}
}

func TestWaitOnlineAlready(t *testing.T) {
	oldOn, oldEx := onlineFiles, extconFiles
	t.Cleanup(func() { onlineFiles, extconFiles = oldOn, oldEx })
	p := filepath.Join(t.TempDir(), "online")
	onlineFiles = []string{p}
	extconFiles = nil
	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WaitOnline(nil); err != nil {
		t.Fatal(err)
	}
}

func TestWaitOnlineNoSysfs(t *testing.T) {
	oldOn, oldEx := onlineFiles, extconFiles
	t.Cleanup(func() { onlineFiles, extconFiles = oldOn, oldEx })
	onlineFiles = []string{filepath.Join(t.TempDir(), "nope")}
	extconFiles = nil
	if err := WaitOnline(nil); err != nil {
		t.Fatal(err)
	}
}

func TestWaitOnlineAppears(t *testing.T) {
	oldOn, oldEx, oldR := onlineFiles, extconFiles, offlineRecheck
	t.Cleanup(func() { onlineFiles, extconFiles, offlineRecheck = oldOn, oldEx, oldR })
	offlineRecheck = 20 * time.Millisecond
	p := filepath.Join(t.TempDir(), "online")
	onlineFiles = []string{p}
	extconFiles = nil
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- WaitOnline(nil) }()
	time.Sleep(5 * time.Millisecond)
	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for VBUS")
	}
}

func TestWaitOnlineStop(t *testing.T) {
	oldOn, oldEx, oldR := onlineFiles, extconFiles, offlineRecheck
	t.Cleanup(func() { onlineFiles, extconFiles, offlineRecheck = oldOn, oldEx, oldR })
	offlineRecheck = 30 * time.Second
	p := filepath.Join(t.TempDir(), "online")
	onlineFiles = []string{p}
	extconFiles = nil
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- WaitOnline(stop) }()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	select {
	case err := <-done:
		if err != ErrStopped {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not wake the VBUS wait")
	}
}

func TestWaitDisconnect(t *testing.T) {
	oldOn, oldEx, oldN := onlineFiles, extconFiles, confirmOffline
	t.Cleanup(func() { onlineFiles, extconFiles, confirmOffline = oldOn, oldEx, oldN })
	confirmOffline = 2
	dir := t.TempDir()
	p := filepath.Join(dir, "online")
	onlineFiles = []string{p}
	extconFiles = nil
	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		WaitDisconnect(nil)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("disconnected while still online")
	default:
	}
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for disconnect")
	}
}

func TestWaitDisconnectAOADip(t *testing.T) {
	oldOn, oldEx, oldN := onlineFiles, extconFiles, confirmOffline
	stop := make(chan struct{})
	exited := make(chan struct{})
	// Stop and join the waiter before restoring the package globals it reads.
	t.Cleanup(func() {
		close(stop)
		<-exited
		onlineFiles, extconFiles, confirmOffline = oldOn, oldEx, oldN
	})
	confirmOffline = 4
	dir := t.TempDir()
	p := filepath.Join(dir, "online")
	onlineFiles = []string{p}
	extconFiles = nil
	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	go func() {
		WaitDisconnect(stop)
		close(exited)
	}()
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond) // one off poll, then recover
	if err := os.WriteFile(p, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	select {
	case <-exited:
		t.Fatal("AOA VBUS dip must not count as unplug")
	default:
	}
}

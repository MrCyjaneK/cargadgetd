//go:build linux

package wifi

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

func Join(ctx context.Context, ssid, pass string) error {
	ssid = strings.TrimSpace(ssid)
	if ssid == "" {
		return fmt.Errorf("wifi: empty ssid")
	}
	if os.Geteuid() == 0 {
		return Connect(ctx, ssid, pass)
	}
	bin := strings.TrimSpace(cfg.Get().Gadget.Bin)
	if bin == "" {
		bin = "/usr/share/cargadgetd/bin/cargadgetd-gadget"
	}
	return joinVia(ctx, bin, ssid, pass)
}

func joinVia(ctx context.Context, bin, ssid, pass string) error {
	cmd := exec.CommandContext(ctx, bin, "-wifi-join")
	cmd.Stdin = bytes.NewReader(writeCred(ssid, pass))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	msg := strings.TrimSpace(buf.String())
	if err != nil {
		if msg == "" {
			return fmt.Errorf("wifi: %s: %w", bin, err)
		}
		if strings.Contains(msg, "wifi:") {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("wifi: %s", msg)
	}
	if msg != "" {
		log.Print(msg)
	}
	return nil
}

func Connect(ctx context.Context, ssid, pass string) error {
	ssid = strings.TrimSpace(ssid)
	if ssid == "" {
		return fmt.Errorf("wifi: empty ssid")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
	}
	if dump, err := gdbus(ctx, "call", "--system", "--dest", "net.connman",
		"--object-path", "/", "--timeout", "10",
		"--method", "net.connman.Manager.GetServices"); err == nil && serviceReady(dump, ssid) {
		return nil
	}
	log.Printf("wifi: waiting for %s", ssid)
	path, err := createService(ctx, ssid, pass)
	if err != nil {
		return err
	}
	// AutoConnect makes ConnMan probe this SSID during a scan. Clear it
	// afterwards so a later appearance does not pull the phone off its
	// normal network.
	defer setAutoConnect(context.Background(), path, false)
	var last error
	logged := false
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		if err := scanWifi(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		last = connectService(ctx, path)
		if !wifiNoCarrier(last) {
			return last
		}
		if !logged {
			log.Printf("wifi: %s not visible yet", ssid)
			logged = true
		}
	}
}

func createService(ctx context.Context, ssid, pass string) (string, error) {
	out, err := gdbus(ctx, "call", "--system", "--dest", "net.connman",
		"--object-path", "/", "--timeout", "35",
		"--method", "net.connman.Manager.CreateService",
		"wifi", "", "", serviceSettings(ssid, pass))
	if err != nil {
		return "", err
	}
	path, ok := objectPath(out)
	if !ok {
		return "", fmt.Errorf("wifi: no service from connman")
	}
	return path, nil
}

// scanWifi blocks in ConnMan until the current scan finishes.
func scanWifi(ctx context.Context) error {
	_, err := gdbus(ctx, "call", "--system", "--dest", "net.connman",
		"--object-path", "/net/connman/technology/wifi", "--timeout", "30",
		"--method", "net.connman.Technology.Scan")
	return err
}

func connectService(ctx context.Context, path string) error {
	_, err := gdbus(ctx, "call", "--system", "--dest", "net.connman",
		"--object-path", path, "--timeout", "35",
		"--method", "net.connman.Service.Connect")
	if err != nil && !strings.Contains(err.Error(), "AlreadyConnected") {
		return err
	}
	return nil
}

func setAutoConnect(ctx context.Context, path string, on bool) {
	val := "<false>"
	if on {
		val = "<true>"
	}
	_, _ = gdbus(ctx, "call", "--system", "--dest", "net.connman",
		"--object-path", path, "--timeout", "10",
		"--method", "net.connman.Service.SetProperty",
		"AutoConnect", val)
}

func gdbus(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gdbus", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if ctx.Err() != nil {
		return out, fmt.Errorf("wifi: %w", ctx.Err())
	}
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			return out, fmt.Errorf("wifi: %w", err)
		}
		return out, fmt.Errorf("wifi: %s", msg)
	}
	return out, nil
}

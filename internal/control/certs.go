package control

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/aaextract"
	"github.com/cargadgetd/cargadgetd/internal/cfg"
	"github.com/cargadgetd/cargadgetd/internal/rgh"
)

func CertsReady() bool {
	return aaextract.ActiveDir(certsBase()) != ""
}

func CertsProbe() string {
	base := certsBase()
	dir := aaextract.ActiveDir(base)
	if dir == "" {
		return "Certificate is missing (" + rgh.ResolvePath(readDaemon().Certs) + ")"
	}
	text, err := aaextract.Probe(dir)
	if err != nil {
		return err.Error()
	}
	return text
}

func CertsDownload() string {
	if err := resetCertsLocation(); err != nil {
		return err.Error()
	}
	base := certsBase()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	_, err := aaextract.InstallDownload(ctx, base)
	if err != nil && !strings.Contains(err.Error(), "already extracted") {
		return err.Error()
	}
	if aaextract.ActiveDir(base) == "" {
		if err != nil {
			return err.Error()
		}
		return "certificate install failed"
	}
	return ""
}

func resetCertsLocation() error {
	return cfg.PatchSectionKey(cfg.UserPath(), "daemon", "certs", strconv.Quote(rgh.ConfigDefault))
}

func CertsInstall(apk string) string {
	if !strings.HasSuffix(strings.ToLower(apk), ".apk") {
		return "Not an apk: " + apk
	}
	base := certsBase()
	if _, err := aaextract.InstallAPK(apk, base); err != nil {
		return err.Error()
	}
	return ""
}

package shell

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

func loadNotifyIcon(name string) *image.RGBA {
	path := resolveNotifyIcon(name)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

func resolveNotifyIcon(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "file://")
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "/") {
		if st, err := os.Stat(name); err == nil && !st.IsDir() {
			return name
		}
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if base == "" || strings.Contains(base, string(filepath.Separator)) {
		return ""
	}
	for _, dir := range []string{
		"/usr/share/themes/sailfish-default/silica/z1.0/icons",
		"/usr/share/icons/hicolor/86x86/apps",
		"/usr/share/icons/hicolor/128x128/apps",
		"/usr/share/icons/hicolor/108x108/apps",
	} {
		p := filepath.Join(dir, base+".png")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

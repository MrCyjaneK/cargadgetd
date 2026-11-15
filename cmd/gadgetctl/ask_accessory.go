package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// linkAskAccessory makes the usb-moded ask gadget answer AOA GET_PROTOCOL.
//
// The C2 gadget is already 18d1:0afe and its kernel setup hook answers those
// requests with accessory.gs2 left unlinked. The JollaPhone2026 gadget is
// 0e8d:0afe and only handles them once accessory.gs2 is in the configuration,
// so DHU stalls, the start flag stays 0, and this process never binds 18d1:2d00.
//
// Returns how many config symlinks were created. Safe to call again: a
// second call is a no-op while the links remain, and recreates them after
// usb-moded replaces the configuration.
func linkAskAccessory() (int, error) {
	g, udc, err := boundGadget()
	if err != nil {
		return 0, err
	}
	// usb-moded sometimes leaves g1 unbound after it rewrites the config
	// (or after a link is removed). An empty UDC is invisible to the host,
	// so GET_PROTOCOL never happens.
	bound := g != ""
	if !bound {
		g = unboundAskGadget()
		if g == "" {
			return 0, nil
		}
		udc = firstUDC()
		if udc == "" {
			return 0, nil
		}
	}
	vid, err := readHexFile(filepath.Join(g, "idVendor"))
	if err != nil {
		return 0, err
	}
	if vid == 0x18d1 {
		return 0, nil
	}
	fns, err := accessoryFunctionNames(g)
	if err != nil {
		return 0, err
	}
	configs, err := configNames(g)
	if err != nil {
		return 0, err
	}
	missing := missingAccessoryLinks(g, configs, fns)
	pkt := filepath.Join(g, "bMaxPacketSize0")
	fixPkt := false
	if n, err := readHexFile(pkt); err == nil && n == 0 {
		fixPkt = true
	}
	if len(missing) == 0 && !fixPkt {
		if !bound {
			if err := os.WriteFile(filepath.Join(g, "UDC"), []byte(udc+"\n"), 0644); err != nil {
				return 0, fmt.Errorf("bind %s: %w", udc, err)
			}
			fmt.Printf("bound %s (%s) so AOA GET_PROTOCOL can be answered\n", filepath.Base(g), udc)
		}
		return 0, nil
	}

	if bound {
		unbindGadget(g)
	}
	rebound := false
	defer func() {
		if rebound {
			return
		}
		if err := os.WriteFile(filepath.Join(g, "UDC"), []byte(udc+"\n"), 0644); err != nil {
			log.Printf("rebind %s: %v", udc, err)
		}
	}()
	if fixPkt {
		if err := os.WriteFile(pkt, []byte("0x40\n"), 0644); err != nil {
			return 0, fmt.Errorf("bMaxPacketSize0: %w", err)
		}
	}
	// configfs looks the symlink target up from the process cwd.
	wd, err := os.Getwd()
	if err != nil {
		return 0, err
	}
	if err := os.Chdir(g); err != nil {
		return 0, err
	}
	defer os.Chdir(wd)
	for _, link := range missing {
		fn := filepath.Base(link)
		if err := os.Symlink(filepath.Join("functions", fn), link); err != nil {
			return 0, fmt.Errorf("link %s: %w", fn, err)
		}
	}
	if err := os.WriteFile(filepath.Join(g, "UDC"), []byte(udc+"\n"), 0644); err != nil {
		return 0, fmt.Errorf("rebind %s: %w", udc, err)
	}
	rebound = true
	if len(missing) > 0 {
		fmt.Printf("linked accessory into %s (%s) so AOA GET_PROTOCOL is answered\n", filepath.Base(g), udc)
	}
	return len(missing), nil
}

// unboundAskGadget is a gadget that has an accessory function and no UDC.
func unboundAskGadget() string {
	ents, err := os.ReadDir(gadgetConfigRoot)
	if err != nil {
		return ""
	}
	var found string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(gadgetConfigRoot, e.Name())
		b, err := os.ReadFile(filepath.Join(p, "UDC"))
		if err != nil || strings.TrimSpace(string(b)) != "" {
			continue
		}
		fns, err := accessoryFunctionNames(p)
		if err != nil || len(fns) == 0 {
			continue
		}
		if found == "" || e.Name() == "g1" {
			found = p
		}
	}
	return found
}

func boundGadget() (string, string, error) {
	ents, err := os.ReadDir(gadgetConfigRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	var g, udc string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(gadgetConfigRoot, e.Name())
		b, err := os.ReadFile(filepath.Join(p, "UDC"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(b))
		if name == "" {
			continue
		}
		if g == "" || e.Name() == "g1" {
			g, udc = p, name
		}
	}
	return g, udc, nil
}

func accessoryFunctionNames(gadget string) ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(gadget, "functions"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "accessory.") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func missingAccessoryLinks(gadget string, configs, fns []string) []string {
	var missing []string
	for _, c := range configs {
		dir := filepath.Join(gadget, "configs", c)
		for _, fn := range fns {
			link := filepath.Join(dir, fn)
			if _, err := os.Lstat(link); err == nil {
				continue
			}
			missing = append(missing, link)
		}
	}
	return missing
}

func readHexFile(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return 0, nil
	}
	return strconv.ParseUint(s, 16, 16)
}

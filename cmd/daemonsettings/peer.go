package main

import (
	"context"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// btPeer is one BlueZ device the settings menu can offer.
type btPeer struct {
	mac, name string
}

var (
	macLead = regexp.MustCompile(`(?i)^[0-9a-f]{2}(?::[0-9a-f]{2}){5}`)
	devPath = regexp.MustCompile(`/org/bluez/hci[0-9]+/dev_([0-9A-Fa-f]{2}(?:_[0-9A-Fa-f]{2}){5})`)
	aliasRe = regexp.MustCompile(`'Alias':\s*<\s*'([^']*)'`)

	listPeers = listBlueZPeers

	peerMu   sync.Mutex
	peerAt   time.Time
	peerHave bool
	peerHit  []btPeer
)

func clearPeerCache() {
	peerMu.Lock()
	peerHave = false
	peerHit = nil
	peerMu.Unlock()
}

func knownPeers() []btPeer {
	peerMu.Lock()
	defer peerMu.Unlock()
	if peerHave && time.Since(peerAt) < 2*time.Second {
		return peerHit
	}
	peerHit = listPeers()
	peerAt = time.Now()
	peerHave = true
	return peerHit
}

// peerEnums is the Head unit menu: none, then each known device as
// "MAC alias". The alias is display only. Commas are spaces so the
// settings row can keep splitting on commas.
func peerEnums() string {
	items := []string{"none"}
	seen := map[string]bool{}
	var peers []btPeer
	for _, p := range knownPeers() {
		mac := canonMAC(p.mac)
		if mac == "" || seen[mac] {
			continue
		}
		seen[mac] = true
		peers = append(peers, btPeer{mac: mac, name: p.name})
	}
	sort.Slice(peers, func(i, j int) bool {
		ni, nj := strings.ToLower(peers[i].name), strings.ToLower(peers[j].name)
		if ni == nj {
			return peers[i].mac < peers[j].mac
		}
		return ni < nj
	})
	for _, p := range peers {
		items = append(items, formatPeer(p))
	}
	if cur := canonMAC(loadCore().peer); cur != "" && !seen[cur] {
		items = append(items, cur)
	}
	return strings.Join(items, ",")
}

func peerChoiceLabel(mac string) string {
	mac = canonMAC(mac)
	if mac == "" {
		return "none"
	}
	for _, item := range strings.Split(peerEnums(), ",") {
		if canonMAC(item) == mac {
			return item
		}
	}
	return mac
}

func peerChoice(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return true
	}
	return canonMAC(v) != ""
}

func savePeer(v string) error {
	v = strings.TrimSpace(v)
	mac := canonMAC(v)
	if v != "" && !strings.EqualFold(v, "none") && mac == "" {
		return errString("head unit must be a bluetooth address")
	}
	return patchSectionKey(userConfigPath(), "daemon", "peer", strconv.Quote(mac))
}

func formatPeer(p btPeer) string {
	name := strings.Join(strings.Fields(strings.ReplaceAll(p.name, ",", " ")), " ")
	if name == "" || strings.EqualFold(name, p.mac) {
		return p.mac
	}
	return p.mac + " " + name
}

func canonMAC(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "none") {
		return ""
	}
	m := macLead.FindString(s)
	if m == "" {
		return ""
	}
	return strings.ToUpper(m)
}

func listBlueZPeers() []btPeer {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gdbus", "call", "--system", "--dest", "org.bluez",
		"--object-path", "/", "--method", "org.freedesktop.DBus.ObjectManager.GetManagedObjects")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseBlueZPeers(string(out))
}

func parseBlueZPeers(dump string) []btPeer {
	locs := devPath.FindAllStringSubmatchIndex(dump, -1)
	var out []btPeer
	for i, loc := range locs {
		if len(loc) < 4 {
			continue
		}
		mac := canonMAC(strings.ReplaceAll(dump[loc[2]:loc[3]], "_", ":"))
		if mac == "" {
			continue
		}
		end := len(dump)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		name := mac
		if m := aliasRe.FindStringSubmatch(dump[loc[0]:end]); len(m) == 2 && strings.TrimSpace(m[1]) != "" {
			name = m[1]
		}
		out = append(out, btPeer{mac: mac, name: name})
	}
	return out
}

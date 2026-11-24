package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBlueZPeers(t *testing.T) {
	dump := `'/org/bluez/hci1/dev_98_1C_A2_E3_98_37': {'org.bluez.Device1': {'Address': <'98:1C:A2:E3:98:37'>, 'Alias': <'AirPods Pro'>, 'Paired': <true>}}, '/org/bluez/hci1/dev_11_22_33_44_55_66': {'org.bluez.Device1': {'Address': <'11:22:33:44:55:66'>, 'Alias': <'LIVI, desk'>}}`
	got := parseBlueZPeers(dump)
	if len(got) != 2 || got[0].mac != "98:1C:A2:E3:98:37" || got[0].name != "AirPods Pro" {
		t.Fatalf("%+v", got)
	}
	if got[1].name != "LIVI, desk" {
		t.Fatalf("name %q", got[1].name)
	}
}

func TestPeerEnumListsDevicesAndSavesMAC(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	prevS, prevU := configSearch, configUser
	prevList := listPeers
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
		listPeers = prevList
		clearPeerCache()
	})
	configUser = user
	configSearch = []string{user}
	listPeers = func() []btPeer {
		return []btPeer{
			{mac: "98:1c:a2:e3:98:37", name: "AirPods Pro"},
			{mac: "11:22:33:44:55:66", name: "LIVI, desk"},
		}
	}
	clearPeerCache()

	row := peerRow(t)
	if coreSettingsEnums(row) != "none,98:1C:A2:E3:98:37 AirPods Pro,11:22:33:44:55:66 LIVI desk" {
		t.Fatalf("enums %q", coreSettingsEnums(row))
	}
	if coreSettingsGetEnum(row) != "none" {
		t.Fatalf("empty peer %q", coreSettingsGetEnum(row))
	}
	if coreSettingsSetEnum(row, "11:22:33:44:55:66 LIVI desk") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().peer != "11:22:33:44:55:66" {
		t.Fatalf("saved %q", loadCore().peer)
	}
	b, err := os.ReadFile(user)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "peer: 11:22:33:44:55:66") && !strings.Contains(string(b), `peer: "11:22:33:44:55:66"`) {
		t.Fatalf("yaml %s", b)
	}
	clearPeerCache()
	if coreSettingsGetEnum(row) != "11:22:33:44:55:66 LIVI desk" {
		t.Fatalf("label %q", coreSettingsGetEnum(row))
	}
	if coreSettingsSetEnum(row, "none") != 0 {
		t.Fatal(coreSettingsError())
	}
	if loadCore().peer != "" {
		t.Fatalf("cleared %q", loadCore().peer)
	}
	if coreSettingsSetEnum(row, "not-a-mac") == 0 {
		t.Fatal("accepted garbage")
	}
}

func TestPeerEnumKeepsConfiguredMAC(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(user, []byte("daemon:\n  peer: \"aa:bb:cc:dd:ee:ff\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	prevList := listPeers
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
		listPeers = prevList
		clearPeerCache()
	})
	configUser = user
	configSearch = []string{user}
	listPeers = func() []btPeer { return nil }
	clearPeerCache()
	row := peerRow(t)
	if coreSettingsEnums(row) != "none,AA:BB:CC:DD:EE:FF" {
		t.Fatalf("enums %q", coreSettingsEnums(row))
	}
	if coreSettingsGetEnum(row) != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("label %q", coreSettingsGetEnum(row))
	}
}

func peerRow(t *testing.T) int {
	t.Helper()
	for i := 0; i < coreSettingsCount(); i++ {
		if coreSettingsLabel(i) == "Head unit" && coreSettingsType(i) == settingEnum {
			return i
		}
	}
	t.Fatal("no head unit row")
	return -1
}

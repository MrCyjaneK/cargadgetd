package wifi

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWifiNoCarrier(t *testing.T) {
	if wifiNoCarrier(nil) {
		t.Fatal("nil")
	}
	if !wifiNoCarrier(fmt.Errorf("wifi: GDBus.Error:net.connman.Error.NoCarrier: No carrier")) {
		t.Fatal("carrier")
	}
	if wifiNoCarrier(fmt.Errorf("wifi: PermissionDenied")) {
		t.Fatal("permission")
	}
}

func TestServiceReady(t *testing.T) {
	dump := "(objectpath '/net/connman/service/wifi_home', {'Name': <'NSA'>, 'State': <'online'>}), (objectpath '/net/connman/service/wifi_pixel', {'Name': <'DIRECT-VU-Pixel9'>, 'State': <'online'>})"
	if !serviceReady(dump, "DIRECT-VU-Pixel9") {
		t.Fatal("pixel")
	}
	if serviceReady(dump, "missing") {
		t.Fatal("missing")
	}
	idle := "(objectpath '/net/connman/service/wifi_pixel', {'Name': <'DIRECT-VU-Pixel9'>, 'State': <'idle'>})"
	if serviceReady(idle, "DIRECT-VU-Pixel9") {
		t.Fatal("idle")
	}
	// Live GetServices: only the first element is tagged objectpath, and an
	// online cellular service sorts ahead of a ready car AP with no internet.
	car := "([(objectpath '/net/connman/service/cellular_1', {'Type': <'cellular'>, 'State': <'online'>, 'Name': <'Orange'>}), ('/net/connman/service/wifi_blow', {'Type': <'wifi'>, 'State': <'ready'>, 'Name': <'BLOW-CarConnect-89EA'>})],)"
	if !serviceReady(car, "BLOW-CarConnect-89EA") {
		t.Fatal("car")
	}
	if !serviceReady(car, "Orange") {
		t.Fatal("cell")
	}
}

func TestServiceSettings(t *testing.T) {
	got := serviceSettings(`car "net"`, `p\ass`)
	want := `[('Name', "car \"net\""), ('SSID', "63617220226E657422"), ('Security', 'psk'), ('Passphrase', "p\\ass"), ('Hidden', 'true'), ('AutoConnect', 'true')]`
	if got != want {
		t.Fatalf("%s", got)
	}
	if _, ok := objectPath("(objectpath '/net/connman/service/wifi_1',)"); !ok {
		t.Fatal("single")
	}
	p, ok := objectPath("(objectpath \"/net/connman/service/wifi_1\",)")
	if !ok || p != "/net/connman/service/wifi_1" {
		t.Fatalf("%q %v", p, ok)
	}
	ssid, pass, err := ReadCred(bytes.NewReader(writeCred("car", "secret")))
	if err != nil || ssid != "car" || pass != "secret" {
		t.Fatalf("%q %q %v", ssid, pass, err)
	}
}

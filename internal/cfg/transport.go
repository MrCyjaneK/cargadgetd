package cfg

import "strings"

type TransportMode struct {
	ID    string
	Title string
	Body  string

	NeedsPeer   bool
	RequirePeer bool
	Hint        string
}

func TransportModes() []TransportMode {
	return []TransportMode{
		{
			ID:    TransportUSB,
			Title: "USB",
			Body: "Connect with a cable\n\n" +
				"WARNING: This will yoink usbmoded and may cause issues (all solved with a restart)\n" +
				"1) Limiting charge capacity won't work\n" +
				"2) USB Developer mode / Mass storage won't work\n" +
				"3) Possibly other issues.\n\n" +
				"This is needed in order to properly handshake with the car.\n" +
				"Please be aware of the issues and when you need USB back just press 'Stop handshake' on the main app page",
		},
		{
			ID:        TransportWireless,
			Title:     "Wireless - developer mode",
			Body:      "Debug only, do not use unless you **really** know what you are doing",
			NeedsPeer: true,
		},
		{
			ID:          TransportWPP,
			Title:       "Bluetooth",
			Body:        "Dial the car over Bluetooth",
			NeedsPeer:   true,
			RequirePeer: true,
			Hint:        "Select the car to continue. Pair it in Bluetooth settings if it is missing.",
		},
	}
}

func NormalizeTransport(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, m := range TransportModes() {
		if v == m.ID {
			return m.ID
		}
	}
	return TransportUSB
}

func ParseTransport(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	for _, m := range TransportModes() {
		if s == m.ID || s == m.Title {
			return m.ID, true
		}
	}
	return "", false
}

func TransportTitle(id string) string {
	id = strings.TrimSpace(id)
	for _, m := range TransportModes() {
		if m.ID == id {
			return m.Title
		}
	}
	return id
}

func TransportTitles() string {
	parts := make([]string, 0, len(TransportModes()))
	for _, m := range TransportModes() {
		parts = append(parts, m.Title)
	}
	return strings.Join(parts, ",")
}

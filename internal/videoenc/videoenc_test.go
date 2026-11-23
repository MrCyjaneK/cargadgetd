package videoenc

import (
	"strings"
	"testing"
)

func TestChoices(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Choices() {
		if c.ID == "" || c.Title == "" {
			t.Fatalf("empty choice %+v", c)
		}
		if strings.Contains(c.Title, ",") {
			t.Fatalf("title %q has a comma; the settings row splits on commas", c.Title)
		}
		if seen[c.ID] {
			t.Fatalf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
		id, ok := Parse(c.Title)
		if !ok || id != c.ID {
			t.Fatalf("title %q -> %q ok=%v", c.Title, id, ok)
		}
		id, ok = Parse(c.ID)
		if !ok || id != c.ID {
			t.Fatalf("id %q not accepted", c.ID)
		}
		if Title(c.ID) != c.Title {
			t.Fatalf("title %s", c.ID)
		}
	}
	if _, ok := Parse("x264"); ok {
		t.Fatal("x264 accepted")
	}
	if Title("custom") != "custom" {
		t.Fatal("unknown id rewritten")
	}
	if !strings.Contains(Titles(), "Hardware (alt)") || !strings.Contains(IDs(), HardwareAlt) {
		t.Fatalf("menu %s ids %s", Titles(), IDs())
	}
}

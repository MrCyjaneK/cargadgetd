package videoenc

import "strings"

const (
	Hardware     = "droidmedia"
	HardwareAlt  = "droidmedia-alt"
	Software     = "software"
	SoftwareFast = "software-fast"
)

type Choice struct {
	ID    string
	Title string
}

func Choices() []Choice {
	return []Choice{
		{ID: Hardware, Title: "Hardware"},
		{ID: HardwareAlt, Title: "Hardware (alt)"},
		{ID: Software, Title: "software (normal)"},
		{ID: SoftwareFast, Title: "software (fast)"},
	}
}

func Title(id string) string {
	id = strings.TrimSpace(id)
	for _, c := range Choices() {
		if c.ID == id {
			return c.Title
		}
	}
	return id
}

func Parse(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	for _, c := range Choices() {
		if s == c.ID || s == c.Title {
			return c.ID, true
		}
	}
	return "", false
}

func Titles() string {
	parts := make([]string, 0, len(Choices()))
	for _, c := range Choices() {
		parts = append(parts, c.Title)
	}
	return strings.Join(parts, ",")
}

func IDs() string {
	parts := make([]string, 0, len(Choices()))
	for _, c := range Choices() {
		parts = append(parts, c.ID)
	}
	return strings.Join(parts, ", ")
}

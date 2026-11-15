package shell

import (
	"strconv"
	"strings"
)

type rawNote struct {
	App, Icon, Title, Body string
	ID                     uint32
}

// parseNotificationList reads gdbus GetNotifications output:
// ([('app', uint32 1, 'icon', 'summary', 'body', ..., {hints}, expire), ...],)
func parseNotificationList(out string) []rawNote {
	var notes []rawNote
	i := 0
	for i < len(out) {
		j := strings.Index(out[i:], "('")
		if j < 0 {
			break
		}
		n, next, ok := parseOneNote(out, i+j)
		if !ok {
			i += j + 1
			continue
		}
		notes = append(notes, n)
		i = next
	}
	return notes
}

func parseOneNote(s string, i int) (rawNote, int, bool) {
	var z rawNote
	if i >= len(s) || s[i] != '(' {
		return z, i, false
	}
	i++
	app, i, ok := readQuoted(s, i)
	if !ok {
		return z, i, false
	}
	i = skipSep(s, i)
	id, i, ok := readUint(s, i)
	if !ok {
		return z, i, false
	}
	i = skipSep(s, i)
	icon, i, ok := readQuoted(s, i)
	if !ok {
		return z, i, false
	}
	i = skipSep(s, i)
	title, i, ok := readQuoted(s, i)
	if !ok {
		return z, i, false
	}
	i = skipSep(s, i)
	body, i, ok := readQuoted(s, i)
	if !ok {
		return z, i, false
	}
	end := i + 2500
	if end > len(s) {
		end = len(s)
	}
	rest := s[i:end]
	if v := hintString(rest, "x-nemo-preview-summary"); v != "" {
		title = v
	}
	if v := hintString(rest, "x-nemo-preview-body"); v != "" {
		body = v
	}
	if v := hintString(rest, "x-nemo-icon"); v != "" && (icon == "" || !strings.HasPrefix(icon, "/")) {
		icon = v
	}
	z = rawNote{App: app, ID: id, Icon: icon, Title: title, Body: body}
	return z, i, true
}

func readQuoted(s string, i int) (string, int, bool) {
	i = skipSpace(s, i)
	if i >= len(s) || s[i] != '\'' {
		return "", i, false
	}
	i++
	var b strings.Builder
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if s[i] == '\'' {
			return b.String(), i + 1, true
		}
		b.WriteByte(s[i])
		i++
	}
	return "", i, false
}

func readUint(s string, i int) (uint32, int, bool) {
	i = skipSpace(s, i)
	if strings.HasPrefix(s[i:], "uint32 ") {
		i += len("uint32 ")
		i = skipSpace(s, i)
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i {
		return 0, i, false
	}
	n, err := strconv.ParseUint(s[i:j], 10, 32)
	if err != nil {
		return 0, i, false
	}
	return uint32(n), j, true
}

func skipSep(s string, i int) int {
	i = skipSpace(s, i)
	if i < len(s) && s[i] == ',' {
		i++
	}
	return skipSpace(s, i)
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

func hintString(rest, key string) string {
	pat := key + "': <'"
	j := strings.Index(rest, pat)
	if j < 0 {
		return ""
	}
	v, _, ok := readQuoted(rest, j+len(pat)-1)
	if !ok {
		return ""
	}
	return v
}

func noteKey(n rawNote) string {
	return n.Title + "\x00" + n.Body
}

func displayTitle(n rawNote) string {
	t := strings.TrimSpace(n.Title)
	if t != "" {
		return t
	}
	return strings.TrimSpace(n.App)
}

func isBlankNote(n rawNote) bool {
	return strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Body) == "" && strings.TrimSpace(n.App) == ""
}

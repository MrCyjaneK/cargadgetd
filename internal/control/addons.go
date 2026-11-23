package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	osReleasePath = "/etc/os-release"
	httpClient    = &http.Client{Timeout: 30 * time.Second}
)

type AddonRow struct {
	Group       string
	Title       string
	Description string
	Icon        string
	URL         string
	Installed   bool
	Update      bool
}

type installedSet struct {
	RPMs  []string
	Files []string
}

var (
	addonMu   sync.Mutex
	addonBody []byte
	addonRows []AddonRow
	addonView []AddonRow
)

func AddonsPresent() bool {
	entries, err := os.ReadDir(PluginDir)
	if err != nil {
		return false
	}
	app, shell := false, false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".so") {
			continue
		}
		if strings.HasPrefix(name, "libcargadgetd_shell_") {
			shell = true
		} else if strings.HasPrefix(name, "libcargadgetd_app_") && name != "libcargadgetd_app_daemon.so" {
			app = true
		}
		if app && shell {
			return true
		}
	}
	return false
}

func AddonsFetch() string {
	index, err := httpGet(RepoRoot)
	if err != nil {
		return "Could not load the repository"
	}
	repo := pickRepo(index, sfosVersion())
	if repo == "" {
		return "No repository for this OS"
	}
	body, err := httpGet(RepoRoot + repo + "/status.json")
	if err != nil {
		return "Could not load the repository"
	}
	rows, err := buildRows([]byte(body), readInstalled())
	if err != nil {
		return "Could not read the repository"
	}
	addonMu.Lock()
	addonBody = []byte(body)
	addonRows = rows
	addonMu.Unlock()
	return ""
}

func AddonsRefresh() string {
	addonMu.Lock()
	body := append([]byte(nil), addonBody...)
	addonMu.Unlock()
	if len(body) == 0 {
		return ""
	}
	rows, err := buildRows(body, readInstalled())
	if err != nil {
		return "Could not read the repository"
	}
	addonMu.Lock()
	addonRows = rows
	addonMu.Unlock()
	return ""
}

func AddonsCount() int {
	addonMu.Lock()
	addonView = append([]AddonRow(nil), addonRows...)
	n := len(addonView)
	addonMu.Unlock()
	return n
}

func AddonAt(i int) (AddonRow, bool) {
	addonMu.Lock()
	defer addonMu.Unlock()
	if i < 0 || i >= len(addonView) {
		return AddonRow{}, false
	}
	return addonView[i], true
}

func httpGet(raw string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "cargadgetd")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func sfosVersion() string {
	b, err := os.ReadFile(osReleasePath)
	if err != nil {
		return ""
	}
	const key = "VERSION_ID="
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key) {
			continue
		}
		v := strings.TrimSpace(line[len(key):])
		v = strings.Trim(v, `"'`)
		return v
	}
	return ""
}

func readInstalled() installedSet {
	var out installedSet
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--qf", "%{NAME} %{VERSION}-%{RELEASE}\n", "cargadgetd*")
	if b, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			sp := strings.IndexByte(line, ' ')
			if sp <= 0 || !strings.HasPrefix(line, "cargadgetd") {
				continue
			}
			out.RPMs = append(out.RPMs, line)
		}
	}
	entries, err := os.ReadDir(PluginDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		app := strings.HasPrefix(name, "libcargadgetd_app_")
		shell := strings.HasPrefix(name, "libcargadgetd_shell_")
		if (app || shell) && strings.HasSuffix(name, ".so") {
			out.Files = append(out.Files, name)
		}
	}
	return out
}

var (
	repoLine = regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+)/$`)
	nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

func pickRepo(indexText, device string) string {
	best := ""
	for _, line := range strings.Split(indexText, "\n") {
		m := repoLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		ver := m[1]
		if device != "" && newer(ver, device) {
			continue
		}
		if best == "" || newer(ver, best) {
			best = ver
		}
	}
	return best
}

func segments(s string) []string {
	parts := nonAlnum.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func newer(a, b string) bool {
	left, right := segments(a), segments(b)
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		if i >= len(left) {
			return false
		}
		if i >= len(right) {
			return true
		}
		anum, bnum := digitsOnly(left[i]), digitsOnly(right[i])
		if anum && bnum {
			av, _ := strconv.Atoi(left[i])
			bv, _ := strconv.Atoi(right[i])
			if av != bv {
				return av > bv
			}
		} else if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return false
}

type rpmFile struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Release  string   `json:"release"`
	Provides []string `json:"provides"`
}

type archRPMs struct {
	list []rpmFile
}

func (a *archRPMs) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("rpms: expected object")
	}
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return err
		}
		var items []rpmFile
		if err := dec.Decode(&items); err != nil {
			return err
		}
		a.list = append(a.list, items...)
	}
	_, err = dec.Token()
	return err
}

type statusDoc struct {
	SFOS    string   `json:"sfos"`
	Recipes []recipe `json:"recipes"`
}

type recipe struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	RPMs        archRPMs `json:"rpms"`
}

func realRpm(r recipe) *rpmFile {
	for i := range r.RPMs.list {
		name := r.RPMs.list[i].Name
		if strings.Contains(name, "debuginfo") || strings.Contains(name, "debugsource") {
			continue
		}
		return &r.RPMs.list[i]
	}
	return nil
}

func oneLine(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
}

func soNames(rpm rpmFile) []string {
	var out []string
	for _, text := range rpm.Provides {
		if !strings.Contains(text, ".so") {
			continue
		}
		if i := strings.IndexByte(text, '('); i >= 0 {
			text = text[:i]
		}
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func rpmVersions(lines []string) map[string]string {
	out := map[string]string{}
	for _, line := range lines {
		sp := strings.IndexByte(line, ' ')
		if sp <= 0 {
			continue
		}
		out[line[:sp]] = line[sp+1:]
	}
	return out
}

func buildRows(body []byte, inst installedSet) ([]AddonRow, error) {
	var doc statusDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var app, shells, apps []AddonRow
	rpms := rpmVersions(inst.RPMs)
	present := map[string]bool{}
	for _, f := range inst.Files {
		present[f] = true
	}
	for _, recipe := range doc.Recipes {
		name := recipe.Name
		rpm := realRpm(recipe)
		if rpm == nil {
			continue
		}
		group := ""
		switch {
		case name == "Cargadgetd":
			group = "Cargadgetd"
		case strings.HasPrefix(name, "Cargadgetd Shell "):
			group = "Shells"
		case strings.HasPrefix(name, "Cargadgetd Plugin "):
			group = "Apps"
		default:
			continue
		}
		repoEvr := rpm.Version + "-" + rpm.Release
		installedEvr := ""
		if rpm.Name != "" {
			installedEvr = rpms[rpm.Name]
		}
		onDisk := false
		for _, lib := range soNames(*rpm) {
			if present[lib] {
				onDisk = true
				break
			}
		}
		isInstalled := onDisk || (group == "Cargadgetd" && installedEvr != "")
		row := AddonRow{
			Group:       group,
			Title:       name,
			Description: oneLine(recipe.Description),
			Icon:        oneLine(recipe.Icon),
			Installed:   isInstalled,
			Update:      isInstalled && installedEvr != "" && newer(repoEvr, installedEvr),
			URL:         RepoRoot + "status#" + doc.SFOS + "/" + url.PathEscape(name),
		}
		switch group {
		case "Cargadgetd":
			app = append(app, row)
		case "Shells":
			shells = append(shells, row)
		default:
			apps = append(apps, row)
		}
	}
	out := make([]AddonRow, 0, len(app)+len(shells)+len(apps))
	out = append(out, app...)
	out = append(out, shells...)
	out = append(out, apps...)
	return out, nil
}

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// configSearch, when non-nil, replaces the packaged/user load list.
// configUser, when set, is the only file these keys are written to.
var (
	configSearch []string
	configUser   string
)

type coreState struct {
	shellID    string
	dropFrames int
	audio      bool
	forcePrime bool
	audioType  string
	channels   int
	tickMs     int
	audioIndex int
	encoder    string
	videoSize  string
	frameMs    int
	live       bool
	videoIndex int
	gap        int
	pcm        string
	ffs        string
	strict     bool
	unhandled  string
	loopback   bool
	sink       string
	player     string
	gps        string
	transport  string
	listen     string
	control    string
	certs      string
	gadgetPath string
	gadgetBin  string
}

func gpsLabel(id string) string {
	if strings.EqualFold(strings.TrimSpace(id), "mock") {
		return "Mock"
	}
	return "GeoClue"
}

func saveGPS(label string) error {
	id := "geoclue"
	if gpsLabel(label) == "Mock" {
		id = "mock"
	}
	return patchSectionKey(userConfigPath(), "gps", "backend", strconv.Quote(id))
}

func loadCore() coreState {
	st := coreState{
		shellID: "basic", dropFrames: 20, audio: true, audioType: "auto", encoder: "droidmedia",
		videoSize: "auto", frameMs: 33, live: true, videoIndex: -1, audioIndex: -1,
		loopback: true, sink: "sink.null", gps: "geoclue",
		transport: "usb",
		listen:    "ffs:///dev/usb-ffs/aa", control: "tcp://127.0.0.1:9277",
		certs:      "/usr/lib/cargadgetd/plugins/reversegearhead.so",
		gadgetPath: "/sys/kernel/config/usb_gadget/aa",
		gadgetBin:  "/usr/share/cargadgetd/bin/cargadgetd-gadget",
	}
	for _, p := range configPaths() {
		applyCore(p, &st)
	}
	if strings.TrimSpace(st.shellID) == "" {
		st.shellID = "basic"
	}
	if strings.TrimSpace(st.encoder) == "" {
		st.encoder = "droidmedia"
	}
	return st
}

func saveShell(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "basic"
	}
	if !validShellID(id) {
		return errString("shell id must be a name")
	}
	return patchSectionKey(userConfigPath(), "shell", "id", strconv.Quote(id))
}

func validShellID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func saveAudio(on bool) error {
	val := "false"
	if on {
		val = "true"
	}
	return patchSectionKey(userConfigPath(), "audio", "enabled", val)
}

func saveLoopback(on bool) error {
	val := "false"
	if on {
		val = "true"
	}
	return patchSectionKey(userConfigPath(), "music", "loopback", val)
}

func saveEncoder(enc string) error {
	if enc != "droidmedia" && enc != "software" {
		return errString("encoder must be droidmedia or software")
	}
	return patchSectionKey(userConfigPath(), "video", "encoder", strconv.Quote(enc))
}

func saveFrame(ms int) error {
	if ms < 16 || ms > 200 {
		return errString("frame_ms must be 16-200")
	}
	return patchSectionKey(userConfigPath(), "video", "frame_ms", strconv.Itoa(ms))
}

type errString string

func (e errString) Error() string { return string(e) }

type coreFile struct {
	Daemon *struct {
		Transport *string `yaml:"transport"`
		Listen    *string `yaml:"listen"`
		Control   *string `yaml:"control"`
		Certs     *string `yaml:"certs"`
	} `yaml:"daemon"`
	Video *struct {
		Encoder     *string `yaml:"encoder"`
		Size        *string `yaml:"size"`
		ConfigIndex *int    `yaml:"config_index"`
		FrameMs     *int    `yaml:"frame_ms"`
		Live        *bool   `yaml:"live"`
	} `yaml:"video"`
	Audio *struct {
		Enabled     *bool   `yaml:"enabled"`
		ForcePrime  *bool   `yaml:"force_prime"`
		TickMs      *int    `yaml:"tick_ms"`
		Channels    *int    `yaml:"channels"`
		ConfigIndex *int    `yaml:"config_index"`
		Type        *string `yaml:"type"`
	} `yaml:"audio"`
	USB *struct {
		MediaGapMs *int    `yaml:"media_gap_ms"`
		PCMDump    *string `yaml:"pcm_dump"`
		FFSDiag    *string `yaml:"ffs_diag"`
	} `yaml:"usb"`
	Gadget *struct {
		Path *string `yaml:"path"`
		Bin  *string `yaml:"bin"`
	} `yaml:"gadget"`
	Shell *struct {
		ID         *string `yaml:"id"`
		DropFrames *int    `yaml:"drop_frames"`
	} `yaml:"shell"`
	Apps *struct {
		Strict       *bool   `yaml:"strict"`
		UnhandledLog *string `yaml:"unhandled_log"`
	} `yaml:"apps"`
	Music *struct {
		Loopback *bool   `yaml:"loopback"`
		Sink     *string `yaml:"sink"`
		Player   *string `yaml:"player"`
	} `yaml:"music"`
	GPS *struct {
		Backend *string `yaml:"backend"`
	} `yaml:"gps"`
}

func applyCore(path string, dst *coreState) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var f coreFile
	if yaml.Unmarshal(b, &f) != nil {
		return
	}
	if f.Daemon != nil {
		if f.Daemon.Transport != nil {
			dst.transport = transportMode(*f.Daemon.Transport)
		}
		if f.Daemon.Listen != nil && strings.TrimSpace(*f.Daemon.Listen) != "" {
			dst.listen = strings.TrimSpace(*f.Daemon.Listen)
		}
		if f.Daemon.Control != nil && strings.TrimSpace(*f.Daemon.Control) != "" {
			dst.control = strings.TrimSpace(*f.Daemon.Control)
		}
		if f.Daemon.Certs != nil && strings.TrimSpace(*f.Daemon.Certs) != "" {
			dst.certs = strings.TrimSpace(*f.Daemon.Certs)
		}
	}
	if f.Shell != nil {
		if f.Shell.ID != nil {
			id := strings.TrimSpace(*f.Shell.ID)
			if id == "" {
				id = "basic"
			}
			dst.shellID = id
		}
		if f.Shell.DropFrames != nil {
			n := *f.Shell.DropFrames
			if n < 0 {
				n = 0
			}
			dst.dropFrames = n
		}
	}
	if f.Audio != nil {
		a := f.Audio
		if a.Enabled != nil {
			dst.audio = *a.Enabled
		}
		if a.ForcePrime != nil {
			dst.forcePrime = *a.ForcePrime
		}
		if a.TickMs != nil {
			dst.tickMs = *a.TickMs
		}
		if a.Channels != nil {
			dst.channels = *a.Channels
		}
		if a.ConfigIndex != nil {
			dst.audioIndex = *a.ConfigIndex
		}
		if a.Type != nil && strings.TrimSpace(*a.Type) != "" {
			dst.audioType = strings.TrimSpace(*a.Type)
		}
	}
	if f.Music != nil {
		if f.Music.Loopback != nil {
			dst.loopback = *f.Music.Loopback
		}
		if f.Music.Sink != nil && strings.TrimSpace(*f.Music.Sink) != "" {
			dst.sink = strings.TrimSpace(*f.Music.Sink)
		}
		if f.Music.Player != nil {
			dst.player = *f.Music.Player
		}
	}
	if f.Video != nil {
		v := f.Video
		if v.Encoder != nil && strings.TrimSpace(*v.Encoder) != "" {
			dst.encoder = strings.TrimSpace(*v.Encoder)
		}
		if v.Size != nil && strings.TrimSpace(*v.Size) != "" {
			dst.videoSize = strings.TrimSpace(*v.Size)
		}
		if v.FrameMs != nil {
			dst.frameMs = *v.FrameMs
		}
		if v.Live != nil {
			dst.live = *v.Live
		}
		if v.ConfigIndex != nil {
			dst.videoIndex = *v.ConfigIndex
		}
	}
	if f.USB != nil {
		if f.USB.MediaGapMs != nil {
			dst.gap = *f.USB.MediaGapMs
		}
		if f.USB.PCMDump != nil {
			dst.pcm = *f.USB.PCMDump
		}
		if f.USB.FFSDiag != nil {
			dst.ffs = *f.USB.FFSDiag
		}
	}
	if f.Gadget != nil {
		if f.Gadget.Path != nil && strings.TrimSpace(*f.Gadget.Path) != "" {
			dst.gadgetPath = strings.TrimSpace(*f.Gadget.Path)
		}
		if f.Gadget.Bin != nil {
			dst.gadgetBin = *f.Gadget.Bin
		}
	}
	if f.Apps != nil {
		if f.Apps.Strict != nil {
			dst.strict = *f.Apps.Strict
		}
		if f.Apps.UnhandledLog != nil {
			dst.unhandled = *f.Apps.UnhandledLog
		}
	}
	if f.GPS != nil && f.GPS.Backend != nil && strings.TrimSpace(*f.GPS.Backend) != "" {
		dst.gps = strings.TrimSpace(*f.GPS.Backend)
	}
}

func configPaths() []string {
	if configSearch != nil {
		return configSearch
	}
	out := []string{
		"/usr/share/cargadgetd/config.yaml",
		"/etc/cargadgetd/config.yaml",
	}
	if p := userConfigPath(); p != "" {
		out = append(out, p)
	}
	return out
}

func userConfigPath() string {
	if configUser != "" {
		return configUser
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "cargadgetd", "config.yaml")
}

func patchSectionKey(path, section, key, val string) error {
	if path == "" {
		return os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := ""
	if err == nil {
		text = strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	if strings.TrimSpace(text) == "" {
		return writeConfig(path, section+":\n  "+key+": "+val+"\n")
	}
	lines := strings.Split(text, "\n")
	secAt := -1
	for i, ln := range lines {
		if topKey(ln) == section {
			secAt = i
			break
		}
	}
	if secAt < 0 {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, section+":", "  "+key+": "+val, "")
		return writeConfig(path, strings.Join(lines, "\n")+"\n")
	}
	end := len(lines)
	found := -1
	for i := secAt + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if indentWidth(lines[i]) == 0 {
			end = i
			break
		}
		if childKey(trim) == key {
			found = i
		}
	}
	if found >= 0 {
		pad := leadingWS(lines[found])
		if pad == "" {
			pad = "  "
		}
		lines[found] = pad + key + ": " + val
	} else {
		line := "  " + key + ": " + val
		lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return writeConfig(path, out)
}

func topKey(line string) string {
	if indentWidth(line) != 0 {
		return ""
	}
	return childKey(strings.TrimSpace(line))
}

func childKey(trim string) string {
	if trim == "" || strings.HasPrefix(trim, "#") {
		return ""
	}
	i := strings.IndexByte(trim, ':')
	if i <= 0 {
		return ""
	}
	return trim[:i]
}

func writeConfig(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cargadgetd-*.yaml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write([]byte(body))
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		if werr != nil {
			return werr
		}
		return cerr
	}
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func indentWidth(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}

func leadingWS(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

package loopback

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

const (
	loopRate     = 48000
	loopChans    = 2
	pactlTimeout = 2 * time.Second
)

func loopbackEnabled() bool {
	c := cfg.Get()
	return c.Audio.Enabled && c.Music.Loopback
}

func loopSinkName() string {
	return strings.TrimSpace(cfg.Get().Music.Sink)
}

type sinkInput struct {
	ID, Sink, Corked, Group, Spec string
	Volume, Mute                  string
}

type volSnap struct {
	vol  string // e.g. "50%" or "50% 50%"
	mute string // "yes" / "no"
}

func volumePercents(line string) string {
	var parts []string
	for i := 0; i < len(line); i++ {
		if line[i] != '%' {
			continue
		}
		j := i
		for j > 0 && line[j-1] >= '0' && line[j-1] <= '9' {
			j--
		}
		if j < i {
			parts = append(parts, line[j:i+1])
		}
	}
	return strings.Join(parts, " ")
}

func volumeStatePath() string {
	if d := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); d != "" {
		return filepath.Join(d, "cargadgetd-music-volume")
	}
	return fmt.Sprintf("/run/user/%d/cargadgetd-music-volume", os.Getuid())
}

func writeVolumeState(s volSnap) {
	if s.vol == "" && s.mute == "" {
		return
	}
	_ = os.WriteFile(volumeStatePath(), []byte(s.vol+"\n"+s.mute+"\n"), 0o600)
}

func parseVolSnapFile(text string) volSnap {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	s := volSnap{}
	if len(lines) > 0 {
		s.vol = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		s.mute = strings.TrimSpace(lines[1])
	}
	return s
}

func readVolumeState() volSnap {
	b, err := os.ReadFile(volumeStatePath())
	if err != nil {
		return volSnap{}
	}
	return parseVolSnapFile(string(b))
}

func clearVolumeState() {
	_ = os.Remove(volumeStatePath())
}

func sinkInputVolumeCmds(id string, snap volSnap) [][]string {
	if id == "" {
		return nil
	}
	var cmds [][]string
	if snap.vol != "" {
		cmds = append(cmds, append([]string{"set-sink-input-volume", id}, strings.Fields(snap.vol)...))
	}
	if snap.mute == "yes" || snap.mute == "no" {
		cmds = append(cmds, []string{"set-sink-input-mute", id, snap.mute})
	}
	return cmds
}

func restoreSinkInputVolume(id string, snap volSnap) {
	for _, args := range sinkInputVolumeCmds(id, snap) {
		_, _ = runPactl(args...)
	}
}

func parseSinkInputs(text string) []sinkInput {
	var out []sinkInput
	var cur *sinkInput
	flush := func() {
		if cur != nil && cur.ID != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "Sink Input #"):
			flush()
			cur = &sinkInput{ID: strings.TrimPrefix(t, "Sink Input #")}
		case cur == nil:
		case strings.HasPrefix(t, "Sink:"):
			cur.Sink = strings.TrimSpace(strings.TrimPrefix(t, "Sink:"))
		case strings.HasPrefix(t, "Corked:"):
			cur.Corked = strings.TrimSpace(strings.TrimPrefix(t, "Corked:"))
		case strings.HasPrefix(t, "Mute:"):
			cur.Mute = strings.TrimSpace(strings.TrimPrefix(t, "Mute:"))
		case strings.HasPrefix(t, "Volume:"):
			cur.Volume = volumePercents(t)
		case strings.HasPrefix(t, "Sample Specification:"):
			cur.Spec = strings.TrimSpace(strings.TrimPrefix(t, "Sample Specification:"))
		case strings.HasPrefix(t, "policy.group ="):
			cur.Group = strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "policy.group =")), `"`)
		}
	}
	flush()
	return out
}

func parseShortSinks(text string) map[string]string {
	idName := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			idName[f[0]] = f[1]
		}
	}
	return idName
}

func pickPlayer(inputs []sinkInput) (id, sinkIdx string, ok bool) {
	for _, in := range inputs {
		if in.Group != "player" {
			continue
		}
		if in.Corked == "yes" {
			continue
		}
		return in.ID, in.Sink, true
	}
	return "", "", false
}

// playersAway is every player stream that is not already on the capture sink,
// corked or not. A corked stream uncorks on that sink, so the next track does
// not start on the speaker.
func playersAway(inputs []sinkInput, sinks map[string]string, nullName string) []sinkInput {
	var out []sinkInput
	for _, in := range inputs {
		if in.Group != "player" || in.ID == "" {
			continue
		}
		name := sinks[in.Sink]
		if name == "" {
			name = in.Sink
		}
		if name == nullName {
			continue
		}
		out = append(out, in)
	}
	return out
}

func sinkInputEvent(line string) bool {
	return strings.Contains(line, "on sink-input ")
}

func hardwareSink(sinks map[string]string, sinkIdx, nullName string) string {
	if name := sinks[sinkIdx]; name != "" && name != nullName {
		return name
	}
	for _, name := range sinks {
		if name == "sink.primary_output" {
			return name
		}
	}
	for _, name := range sinks {
		if name != "" && name != nullName {
			return name
		}
	}
	return ""
}

func parsePactlInfo(text string) (sink, source string) {
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "Default Sink:"):
			sink = strings.TrimSpace(strings.TrimPrefix(t, "Default Sink:"))
		case strings.HasPrefix(t, "Default Source:"):
			source = strings.TrimSpace(strings.TrimPrefix(t, "Default Source:"))
		}
	}
	return sink, source
}

func preferredHardwareSink(sinks map[string]string, defaultSink, nullName string) string {
	if defaultSink != "" && defaultSink != nullName {
		return defaultSink
	}
	return hardwareSink(sinks, "", nullName)
}

func preferredHardwareSource(sources map[string]string, defaultSrc string) string {
	if defaultSrc != "" && defaultSrc != "source.null" && !strings.HasSuffix(defaultSrc, ".monitor") {
		return defaultSrc
	}
	for _, name := range sources {
		if name == "source.primary_input" {
			return name
		}
	}
	for _, name := range sources {
		if strings.Contains(name, "input") && !strings.HasSuffix(name, ".monitor") {
			return name
		}
	}
	return ""
}

func inputsOnSink(inputs []sinkInput, sinks map[string]string, sinkName string) []string {
	var ids []string
	for _, in := range inputs {
		if sinks[in.Sink] == sinkName || in.Sink == sinkName {
			ids = append(ids, in.ID)
		}
	}
	return ids
}

func healPlayback(origSink, origSource, movedID string, snap volSnap) {
	if snap.vol == "" && snap.mute == "" {
		snap = readVolumeState()
	}
	sinksOut, err := runPactl("list", "short", "sinks")
	if err != nil {
		return
	}
	sinks := parseShortSinks(sinksOut)
	nullName := loopSinkName()
	hw := origSink
	if hw == "" || hw == nullName {
		hw = preferredHardwareSink(sinks, "", nullName)
	}
	if hw == "" {
		hw = "sink.primary_output"
	}

	restored := map[string]bool{}
	moveOffNull := func() {
		inOut, _ := runPactl("list", "sink-inputs")
		for _, id := range inputsOnSink(parseSinkInputs(inOut), sinks, nullName) {
			_, _ = runPactl("move-sink-input", id, hw)
			if !restored[id] {
				restored[id] = true
				restoreSinkInputVolume(id, snap)
			}
		}
	}

	// Move players off sink.null before touching defaults. set-default-sink
	// while parec still holds the monitor lets Pulse adopt null as default.
	moveOffNull()
	_, _ = runPactl("set-default-sink", hw)

	infoOut, _ := runPactl("info")
	_, curSrc := parsePactlInfo(infoOut)
	if curSrc == "source.null" || curSrc == "" {
		srcsOut, _ := runPactl("list", "short", "sources")
		src := origSource
		if src == "" || src == "source.null" || strings.HasSuffix(src, ".monitor") {
			src = preferredHardwareSource(parseShortSinks(srcsOut), origSource)
		}
		if src != "" {
			_, _ = runPactl("set-default-source", src)
		}
	}

	moveOffNull()
	if movedID != "" && !restored[movedID] {
		restoreSinkInputVolume(movedID, snap)
	}
	clearVolumeState()
	fmt.Fprintf(os.Stderr, "loopback: heal sink=%s\n", hw)
}

func monitorSource(sources map[string]string, sink string) (string, bool) {
	want := sink + ".monitor"
	for _, name := range sources {
		if name == want {
			return name, true
		}
	}
	return "", false
}

func isMonitorName(name string) bool {
	return strings.HasSuffix(name, ".monitor")
}

type sourceOutput struct {
	ID, Source, PID, App, Spec string
}

func parseSourceOutputs(text string) []sourceOutput {
	var out []sourceOutput
	var cur *sourceOutput
	flush := func() {
		if cur != nil && cur.ID != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "Source Output #"):
			flush()
			cur = &sourceOutput{ID: strings.TrimPrefix(t, "Source Output #")}
		case cur == nil:
		case strings.HasPrefix(t, "Source:"):
			cur.Source = strings.TrimSpace(strings.TrimPrefix(t, "Source:"))
		case strings.HasPrefix(t, "Sample Specification:"):
			cur.Spec = strings.TrimSpace(strings.TrimPrefix(t, "Sample Specification:"))
		case strings.HasPrefix(t, "application.process.id ="):
			cur.PID = strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "application.process.id =")), `"`)
		case strings.HasPrefix(t, "application.name ="):
			cur.App = strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "application.name =")), `"`)
		}
	}
	flush()
	return out
}

func pickSourceOutputByPID(outs []sourceOutput, pid int) (sourceOutput, bool) {
	want := strconv.Itoa(pid)
	for _, o := range outs {
		if o.PID == want {
			return o, true
		}
	}
	return sourceOutput{}, false
}

func pulseChildEnv(src string) []string {
	var out []string
	for _, e := range pulseEnv() {
		if strings.HasPrefix(e, "PULSE_SOURCE=") || strings.HasPrefix(e, "PULSE_SINK=") {
			continue
		}
		out = append(out, e)
	}
	if src != "" {
		out = append(out, "PULSE_SOURCE="+src)
	}
	return out
}

func pcmS16LE(b []byte) []int16 {
	n := len(b) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2 : i*2+2]))
	}
	return out
}

type pulseLoopback struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	orig    string
	origSrc string
	opened  bool
	lastErr string
	wantOff bool
	halted  bool
}

func newPulseLoopback() *pulseLoopback {
	if !loopbackEnabled() {
		return nil
	}
	if _, err := exec.LookPath("pactl"); err != nil {
		return nil
	}
	if _, err := exec.LookPath("parec"); err != nil {
		return nil
	}
	return &pulseLoopback{}
}

func (p *pulseLoopback) Run(stop <-chan struct{}) {
	if p == nil {
		return
	}
	p.ensure()
	events := make(chan struct{}, 1)
	go p.subscribe(stop, events)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			p.ensure()
			p.adopt()
		case <-events:
			p.adopt()
		}
	}
}

func (p *pulseLoopback) subscribe(stop <-chan struct{}, events chan<- struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		cmd := exec.Command("pactl", "subscribe")
		cmd.Env = pulseEnv()
		out, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			select {
			case <-stop:
				return
			case <-time.After(time.Second):
				continue
			}
		}
		done := make(chan struct{})
		go func() {
			select {
			case <-stop:
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			case <-done:
			}
		}()
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if sinkInputEvent(sc.Text()) {
				select {
				case events <- struct{}{}:
				default:
				}
			}
		}
		close(done)
		_ = cmd.Wait()
		select {
		case <-stop:
			return
		case <-time.After(time.Second):
		}
	}
}

func (p *pulseLoopback) Halt() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.halted = true
	p.wantOff = true
	cmd := p.cmd
	p.cmd = nil
	orig, origSrc := p.orig, p.origSrc
	p.mu.Unlock()
	// Kill parec and join capture first so Pulse is not holding
	// sink.null.monitor when we move streams back to the speaker.
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	stopPCMCapture()
	healPlayback(orig, origSrc, "", volSnap{})
}

func (p *pulseLoopback) ensure() {
	p.mu.Lock()
	if p.halted || p.wantOff {
		p.mu.Unlock()
		return
	}
	opened := p.opened
	running := p.cmd != nil
	p.mu.Unlock()
	if !opened {
		p.note(p.open())
		return
	}
	if !running {
		p.note(p.startCapture())
	}
}

func (p *pulseLoopback) note(err error) {
	if err == nil {
		p.mu.Lock()
		p.lastErr = ""
		p.mu.Unlock()
		return
	}
	msg := err.Error()
	p.mu.Lock()
	dup := p.lastErr == msg
	p.lastErr = msg
	p.mu.Unlock()
	if !dup {
		fmt.Fprintf(os.Stderr, "loopback: %v\n", err)
	}
}

func (p *pulseLoopback) open() error {
	p.mu.Lock()
	if p.halted || p.wantOff || p.opened {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	sinksOut, err := runPactl("list", "short", "sinks")
	if err != nil {
		return err
	}
	sinks := parseShortSinks(sinksOut)
	want := loopSinkName()
	found := false
	for _, name := range sinks {
		if name == want {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("pulse: no %s sink", want)
	}
	infoOut, _ := runPactl("info")
	defSink, defSrc := parsePactlInfo(infoOut)
	srcsOut, _ := runPactl("list", "short", "sources")
	orig := preferredHardwareSink(sinks, defSink, want)
	origSrc := preferredHardwareSource(parseShortSinks(srcsOut), defSrc)
	if defSink == want {
		healPlayback(orig, origSrc, "", volSnap{})
	}
	_, _ = runPactl("set-sink-mute", want, "0")
	_, _ = runPactl("set-sink-volume", want, "100%")

	p.mu.Lock()
	p.orig, p.origSrc = orig, origSrc
	p.opened = true
	halted := p.halted || p.wantOff
	p.mu.Unlock()
	if halted {
		return nil
	}
	if err := p.startCapture(); err != nil {
		return err
	}
	p.adopt()
	return nil
}

func (p *pulseLoopback) adopt() {
	p.mu.Lock()
	if p.halted || p.wantOff || !p.opened {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	sinksOut, err := runPactl("list", "short", "sinks")
	if err != nil {
		return
	}
	sinks := parseShortSinks(sinksOut)
	want := loopSinkName()
	inOut, err := runPactl("list", "sink-inputs")
	if err != nil {
		return
	}
	away := playersAway(parseSinkInputs(inOut), sinks, want)
	wrote := false
	for _, in := range away {
		if !wrote && (in.Volume != "" || in.Mute != "") {
			writeVolumeState(volSnap{vol: in.Volume, mute: in.Mute})
			wrote = true
		}
		if _, err := runPactl("move-sink-input", in.ID, want); err != nil {
			fmt.Fprintf(os.Stderr, "loopback: move #%s: %v\n", in.ID, err)
			continue
		}
		_, _ = runPactl("set-sink-input-mute", in.ID, "0")
		_, _ = runPactl("set-sink-input-volume", in.ID, "100%")
		fmt.Fprintf(os.Stderr, "loopback: sink-input #%s -> %s\n", in.ID, want)
	}
}

func (p *pulseLoopback) startCapture() error {
	p.mu.Lock()
	if p.cmd != nil || p.halted || p.wantOff {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	want := loopSinkName()
	src := ""
	if srcsOut, err := runPactl("list", "short", "sources"); err == nil {
		if name, ok := monitorSource(parseShortSinks(srcsOut), want); ok {
			src = name
		}
	}
	if src == "" || !isMonitorName(src) {
		return fmt.Errorf("pulse: no %s.monitor source", want)
	}

	var pipefds [2]int
	if err := syscall.Pipe(pipefds[:]); err != nil {
		return err
	}
	r, w := pipefds[0], pipefds[1]
	_ = syscall.SetNonblock(r, false)
	_ = syscall.SetNonblock(w, false)

	cmd := exec.Command("parec",
		"-d", src,
		"--client-name", "cargadgetd-music",
		"--stream-name", "loopback",
		"--format", "s16le",
		"--rate", strconv.Itoa(loopRate),
		"--channels", strconv.Itoa(loopChans),
		"--raw",
		"--property", "module-stream-restore.id=cargadgetd-music-loopback",
		"--latency-msec", "20")
	cmd.Env = pulseChildEnv(src)
	pw := os.NewFile(uintptr(w), "parec-stdout")
	cmd.Stdout = pw
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = syscall.Close(r)
		return err
	}
	_ = pw.Close()

	if err := pinSourceOutput(cmd.Process.Pid, src); err != nil {
		_ = syscall.Close(r)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return err
	}

	p.mu.Lock()
	if p.wantOff || p.halted {
		p.mu.Unlock()
		_ = syscall.Close(r)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return nil
	}
	p.cmd = cmd
	p.mu.Unlock()

	fmt.Fprintf(os.Stderr, "loopback: record %s\n", src)
	startPCMCapture(r)
	go func() {
		err := cmd.Wait()
		stopPCMCapture()
		p.mu.Lock()
		if p.cmd == cmd {
			p.cmd = nil
		}
		p.mu.Unlock()
		if err != nil && !p.stopped() {
			fmt.Fprintf(os.Stderr, "loopback: parec: %v\n", err)
		}
	}()
	return nil
}

func (p *pulseLoopback) stopped() bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.halted || p.wantOff
}

func pinSourceOutput(pid int, wantSrc string) error {
	if pid <= 0 {
		return fmt.Errorf("pulse: parec has no pid")
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	var last string
	for {
		out, err := runPactl("list", "source-outputs")
		if err != nil {
			return err
		}
		so, ok := pickSourceOutputByPID(parseSourceOutputs(out), pid)
		if ok {
			srcsOut, _ := runPactl("list", "short", "sources")
			name := parseShortSinks(srcsOut)[so.Source]
			last = name
			if name == wantSrc {
				fmt.Fprintf(os.Stderr, "loopback: parec source-output #%s spec=%q source=%s\n", so.ID, so.Spec, name)
				return nil
			}
			if _, err := runPactl("move-source-output", so.ID, wantSrc); err != nil {
				return fmt.Errorf("pulse: move-source-output #%s -> %s: %w", so.ID, wantSrc, err)
			}
		}
		if time.Now().After(deadline) {
			if last != "" {
				return fmt.Errorf("pulse: parec stuck on %s, want %s", last, wantSrc)
			}
			return fmt.Errorf("pulse: no source-output for parec pid %d", pid)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func runPactl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pactlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pactl", args...)
	cmd.Env = pulseEnv()
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("pactl timed out")
	}
	if err != nil {
		return out, fmt.Errorf("pactl %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func pulseEnv() []string {
	// pactl labels are gettext'd. Polish "odpływ wejścia" never matches
	// "Sink Input #", so the player stays on the phone speaker.
	env := withCLocale(os.Environ())
	if strings.TrimSpace(os.Getenv("PULSE_SERVER")) != "" || strings.TrimSpace(os.Getenv("PULSE_RUNTIME_PATH")) != "" {
		return env
	}
	uid := os.Getuid()
	candidates := []string{
		fmt.Sprintf("/run/user/%d/pulse/native", uid),
		"/run/user/100000/pulse/native",
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		candidates = append([]string{xdg + "/pulse/native"}, candidates...)
	}
	for _, sock := range candidates {
		if st, err := os.Stat(sock); err == nil && (st.Mode()&os.ModeSocket != 0 || st.Mode()&os.ModeNamedPipe != 0) {
			return append(env, "PULSE_SERVER=unix:"+sock)
		}
		if _, err := os.Stat(sock); err == nil {
			return append(env, "PULSE_SERVER=unix:"+sock)
		}
	}
	return env
}

func withCLocale(env []string) []string {
	out := make([]string, 0, len(env)+3)
	for _, e := range env {
		if isLocaleEnv(e) {
			continue
		}
		out = append(out, e)
	}
	return append(out, "LANG=C", "LC_ALL=C", "LC_MESSAGES=C")
}

func isLocaleEnv(e string) bool {
	return strings.HasPrefix(e, "LANG=") ||
		strings.HasPrefix(e, "LC_ALL=") ||
		strings.HasPrefix(e, "LC_MESSAGES=") ||
		strings.HasPrefix(e, "LANGUAGE=")
}

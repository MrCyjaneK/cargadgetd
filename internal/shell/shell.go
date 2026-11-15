package shell

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cargadgetd/cargadgetd/internal/gps"
	"github.com/cargadgetd/cargadgetd/internal/livevid"
)

// Shell is the session host: app plugins, mixer, and the shell .so.
// Chrome (grid, recents, clock, banner) lives in libcargadgetd_shell_<id>.so.
type Shell struct {
	mu          sync.Mutex
	shellMu     sync.Mutex
	w, h        int
	apps        []*App
	byID        map[string]*App
	mix         *Mixer
	framePeriod time.Duration

	started bool
	closed  bool
	loadWG  sync.WaitGroup

	shellID  string
	shellErr string // OpenShell failure; latched until a later open works
	plug     shellSurface
	ready    atomic.Bool

	// Shell plugin draw runs off the encode tick. drawKick is buffered 1 so
	// a tick never blocks and a stuck draw is left running.
	drawKick  chan struct{}
	drawStop  chan struct{}
	drawDone  chan struct{}
	asyncDraw atomic.Bool

	frameMu    sync.Mutex
	published  *image.RGBA
	drawStart  time.Time
	drawGen    uint64
	drawEpoch  uint64
	dropFrames int
	dropped    int
	dropEpoch  uint64
	drawAt     time.Time

	noteStop         chan struct{}
	pendingApps      []*App
	pendingTouch     [][3]int
	pendingNotes     []pendingNote
	pendingKeys      []keyEv
	pendingShellKeys []keyEv
	pendingShellGPS  []shellGPSEv

	gpsSrc   gps.Source
	gpsStop  chan struct{}
	gpsDone  chan struct{}
	gpsHave  bool
	gpsGen   int
	gpsLast  gps.Fix
	gpsNotes []gps.SatNote
	gpsShell []gps.SatNote
	gpsSent  map[*App]int
	shellGen int
}

const (
	DefaultFramePeriod = 33 * time.Millisecond
	MinFramePeriod     = 16 * time.Millisecond
	// DefaultDropFrames is how many encode ticks may pass while one plug.draw
	// is still running before the frame says the shell is not responding.
	// 0 leaves the last picture up.
	DefaultDropFrames = 20
)

func ClampFramePeriod(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultFramePeriod
	}
	if d < MinFramePeriod {
		return MinFramePeriod
	}
	return d
}

type pendingNote struct {
	title, body string
	icon        *image.RGBA
}

type keyEv struct {
	name    string
	pressed int
}

// App is one loaded plugin. The shell .so draws it.
type App struct {
	ID, Name string
	Icon     *image.RGBA
	runner   Runner
	starting bool
	inited   bool
	pushed   bool
}

// Runner is a .so app. Draw, touch, pause, and resume are invoked by the
// shell plugin on the shell draw goroutine, not by the host.
type Runner interface {
	Start(play func(pcm []int16, nframes, ch, rate int))
	Stop()
	HuInput(name string, pressed int)
	GPSFix(hasPos int, lat, lon, accM float64, hasSpeed, kmh, hasCourse int, course float64, err string)
	GPSSat(gain int, kicker, title, sub, detail, flag string)
	AppPtrs() (draw, touch, pause, resume unsafe.Pointer)
}

type shellSurface interface {
	init(frameMS, w, h int)
	pushApp(a *App)
	notif(title, body string, icon *image.RGBA)
	draw(dst *image.RGBA, now time.Time)
	touch(x, y, action int)
	huinput(event string, pressed int)
	stop()
}

func New(w, h, audioRate, audioChans int) *Shell {
	if w < 1 {
		w = livevid.DefaultWidth
	}
	if h < 1 {
		h = livevid.DefaultHeight
	}
	return &Shell{
		w:           w,
		h:           h,
		byID:        make(map[string]*App),
		mix:         NewMixer(audioRate, audioChans),
		framePeriod: DefaultFramePeriod,
		shellID:     "basic",
		dropFrames:  DefaultDropFrames,
	}
}

// SetDropFrames sets how many encode ticks one in-flight plug.draw may span
// before Draw paints the not-responding line. 0 disables that line.
func (s *Shell) SetDropFrames(n int) {
	if n < 0 {
		n = 0
	}
	s.frameMu.Lock()
	s.dropFrames = n
	s.frameMu.Unlock()
}

func (s *Shell) Mixer() *Mixer { return s.mix }

func (s *Shell) SetFramePeriod(d time.Duration) {
	s.mu.Lock()
	s.framePeriod = ClampFramePeriod(d)
	ms := int(s.framePeriod / time.Millisecond)
	s.mu.Unlock()
	setHostFrameMs(ms)
}

func (s *Shell) FramePeriod() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.framePeriod <= 0 {
		return DefaultFramePeriod
	}
	return s.framePeriod
}

func (s *Shell) playPCM(pcm []int16, nframes, ch, rate int) {
	s.mix.Play(pcm, nframes, ch, rate)
}

// OpenShell dlopens libcargadgetd_shell_<id>.so. id empty or invalid is basic.
// A missing library is painted as an error line; the encode tick does not wait.
func (s *Shell) OpenShell(dir, id string) error {
	id = NormalizeShellID(id)
	s.mu.Lock()
	s.shellID = id
	w, h := s.w, s.h
	ms := int(s.framePeriod / time.Millisecond)
	if ms <= 0 {
		ms = int(DefaultFramePeriod / time.Millisecond)
	}
	s.mu.Unlock()
	setHostFrameMs(ms)
	path := filepath.Join(dir, "libcargadgetd_shell_"+id+".so")
	plug, err := openShellPlugin(path, id)
	if err != nil {
		s.mu.Lock()
		s.shellErr = err.Error()
		s.plug = nil
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.plug = plug
	s.shellErr = ""
	done := s.armDrawLocked()
	s.mu.Unlock()
	if done == nil {
		return fmt.Errorf("shell: draw loop already started")
	}
	log.Printf("shell: %s (%s)", id, path)
	go func() {
		defer close(done)
		s.shellMu.Lock()
		plug.init(ms, w, h)
		s.shellMu.Unlock()
		s.ready.Store(true)
		s.drawLoop(plug)
	}()
	return nil
}

// armDrawLocked prepares the off-tick draw loop. Caller holds s.mu.
// The caller closes the returned channel when the goroutine exits.
func (s *Shell) armDrawLocked() chan struct{} {
	if s.drawDone != nil {
		return nil
	}
	s.drawKick = make(chan struct{}, 1)
	s.drawStop = make(chan struct{})
	s.drawDone = make(chan struct{})
	s.asyncDraw.Store(true)
	return s.drawDone
}

// WaitShell reports whether shell_init has returned. A missing plugin is not ready.
func (s *Shell) WaitShell(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if s.ready.Load() {
			return true
		}
		s.mu.Lock()
		missing := s.plug == nil
		s.mu.Unlock()
		if missing || time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func NormalizeShellID(id string) string {
	id = strings.TrimSpace(id)
	if validAppID(id) {
		return id
	}
	return "basic"
}

func (s *Shell) StartApps() {
	setHostPlay(s.playPCM)
	setHostFrameMs(int(s.FramePeriod() / time.Millisecond))
	s.mu.Lock()
	s.started = true
	apps := append([]*App(nil), s.apps...)
	s.mu.Unlock()
	for _, a := range apps {
		s.startOne(a)
	}
}

func (s *Shell) startOne(a *App) {
	if a == nil {
		return
	}
	s.mu.Lock()
	if s.closed || a.inited || a.starting || a.runner == nil {
		s.mu.Unlock()
		return
	}
	a.starting = true
	s.mu.Unlock()
	a.runner.Start(s.playPCM)
	s.mu.Lock()
	a.inited = true
	s.mu.Unlock()
	s.enqueueApp(a)
}

func (s *Shell) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	stop := s.noteStop
	s.noteStop = nil
	gpsStop := s.gpsStop
	gpsDone := s.gpsDone
	gpsSrc := s.gpsSrc
	s.gpsStop = nil
	s.gpsDone = nil
	s.gpsSrc = nil
	apps := append([]*App(nil), s.apps...)
	plug := s.plug
	stopDraw := s.drawStop
	drawDone := s.drawDone
	s.mu.Unlock()
	if gpsStop != nil {
		close(gpsStop)
	}
	if gpsDone != nil {
		<-gpsDone
	}
	if gpsSrc != nil {
		gpsSrc.Close()
	}
	if stop != nil {
		close(stop)
	}
	for _, a := range apps {
		if a.runner != nil {
			a.runner.Stop()
		}
	}
	if stopDraw != nil {
		close(stopDraw)
	}
	if drawDone != nil {
		<-drawDone
	}
	if plug != nil {
		s.shellMu.Lock()
		plug.stop()
		s.shellMu.Unlock()
	}
}

// WaitLoad blocks until background plugin dlopen finishes, or d elapses.
// d<=0 waits forever. Returns false on timeout.
func (s *Shell) WaitLoad(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.loadWG.Wait()
		close(done)
	}()
	if d <= 0 {
		<-done
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-done:
		return true
	case <-t.C:
		return false
	}
}

func (s *Shell) AppIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, len(s.apps))
	for i, a := range s.apps {
		ids[i] = a.ID
	}
	return ids
}

// attach adds an app after dlopen. Safe from the loader goroutine.
func (s *Shell) attach(a *App) {
	if a == nil || a.ID == "" {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if _, ok := s.byID[a.ID]; ok {
		s.mu.Unlock()
		return
	}
	s.apps = append(s.apps, a)
	s.byID[a.ID] = a
	started := s.started
	s.mu.Unlock()
	if started {
		s.startOne(a)
	}
}

func (s *Shell) enqueueApp(a *App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || a == nil || a.pushed {
		return
	}
	a.pushed = true
	s.pendingApps = append(s.pendingApps, a)
}

func (s *Shell) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.byID[id]
	return ok
}

// Touch queues a canvas-space event for the shell. Flushed on the shell draw goroutine.
func (s *Shell) Touch(x, y uint32, action int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.pendingTouch = append(s.pendingTouch, [3]int{int(x), int(y), action})
	if len(s.pendingTouch) > 256 {
		s.pendingTouch = s.pendingTouch[len(s.pendingTouch)-256:]
	}
}

// HuInput queues a named HU key. Draw delivers it to every inited app.
// The shell plugin receives it on the draw goroutine.
func (s *Shell) HuInput(name string, pressed int) {
	if name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.pendingKeys = append(s.pendingKeys, keyEv{name: name, pressed: pressed})
	if len(s.pendingKeys) > 64 {
		s.pendingKeys = s.pendingKeys[len(s.pendingKeys)-64:]
	}
}

func (s *Shell) showNote(title, body string, icon *image.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.pendingNotes = append(s.pendingNotes, pendingNote{title: title, body: body, icon: icon})
	if len(s.pendingNotes) > 8 {
		s.pendingNotes = s.pendingNotes[len(s.pendingNotes)-8:]
	}
}

// Draw delivers queued keys to apps and paints dst without waiting on the
// shell plugin. Until shell_init returns, the frame is black with "init - <id>".
// A shell that failed to open is an error line. A plug.draw that is still
// running after dropFrames encode ticks is the not-responding line.
func (s *Shell) Draw(dst *image.RGBA, now time.Time) {
	keys := s.takeKeys()
	for _, k := range keys {
		s.broadcastKey(k)
	}
	if s.asyncDraw.Load() {
		s.enqueueShellKeys(keys)
	}
	s.flushGPS()
	if dst == nil || dst.Bounds().Empty() {
		return
	}
	s.mu.Lock()
	plug := s.plug
	id := s.shellID
	errMsg := s.shellErr
	s.mu.Unlock()
	if plug == nil {
		if errMsg != "" {
			drawShellError(dst, id, "did not load")
			return
		}
		drawInitPlaceholder(dst, id)
		return
	}
	if !s.ready.Load() {
		drawInitPlaceholder(dst, id)
		return
	}
	s.mu.Lock()
	s.drawAt = now
	s.mu.Unlock()
	s.kickDraw()
	if hung, copied := s.takeFrame(dst); hung {
		drawShellError(dst, id, "not responding")
		return
	} else if !copied {
		drawInitPlaceholder(dst, id)
	}
}

func (s *Shell) enqueueShellKeys(keys []keyEv) {
	if len(keys) == 0 {
		return
	}
	s.mu.Lock()
	s.pendingShellKeys = append(s.pendingShellKeys, keys...)
	if len(s.pendingShellKeys) > 64 {
		s.pendingShellKeys = s.pendingShellKeys[len(s.pendingShellKeys)-64:]
	}
	s.mu.Unlock()
}

func (s *Shell) takeShellKeys() []keyEv {
	s.mu.Lock()
	defer s.mu.Unlock()
	ks := s.pendingShellKeys
	s.pendingShellKeys = nil
	return ks
}

func (s *Shell) kickDraw() {
	s.mu.Lock()
	ch := s.drawKick
	s.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *Shell) drawLoop(plug shellSurface) {
	buf := image.NewRGBA(image.Rect(0, 0, s.w, s.h))
	for {
		select {
		case <-s.drawStop:
			return
		case <-s.drawKick:
		}
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return
		}
		s.paintPlug(plug, buf)
	}
}

func (s *Shell) paintPlug(plug shellSurface, buf *image.RGBA) {
	apps := s.takeApps()
	touches := s.takeTouches()
	notes := s.takeNotes()
	keys := s.takeShellKeys()
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	s.frameMu.Lock()
	s.drawEpoch++
	s.drawStart = time.Now()
	s.frameMu.Unlock()
	defer func() {
		s.frameMu.Lock()
		s.drawStart = time.Time{}
		s.frameMu.Unlock()
	}()
	for _, a := range apps {
		plug.pushApp(a)
	}
	for _, t := range touches {
		plug.touch(t[0], t[1], t[2])
	}
	for _, n := range notes {
		plug.notif(n.title, n.body, n.icon)
	}
	for _, k := range keys {
		plug.huinput(k.name, k.pressed)
	}
	s.deliverShellGPS(plug)
	s.mu.Lock()
	at := s.drawAt
	s.mu.Unlock()
	if at.IsZero() {
		at = time.Now()
	}
	plug.draw(buf, at)
	s.frameMu.Lock()
	if s.published == nil || !s.published.Bounds().Eq(buf.Bounds()) {
		s.published = image.NewRGBA(buf.Bounds())
	}
	copy(s.published.Pix, buf.Pix)
	s.drawGen++
	s.drawStart = time.Time{}
	s.frameMu.Unlock()
}

// takeFrame copies the last shell frame into dst. hung means this plug.draw
// has already spanned dropFrames encode ticks; the call is left running.
// dropFrames 0 never reports hung.
func (s *Shell) takeFrame(dst *image.RGBA) (hung, copied bool) {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	if s.drawStart.IsZero() {
		s.dropped = 0
		s.dropEpoch = 0
	} else {
		if s.dropEpoch != s.drawEpoch {
			s.dropEpoch = s.drawEpoch
			s.dropped = 1
		} else {
			s.dropped++
		}
		if s.dropFrames > 0 && s.dropped >= s.dropFrames {
			return true, false
		}
	}
	if s.published == nil {
		return false, false
	}
	draw.Draw(dst, dst.Bounds(), s.published, s.published.Bounds().Min, draw.Src)
	return false, true
}

func (s *Shell) publishGen() uint64 {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	return s.drawGen
}

func (s *Shell) waitPublished(after uint64, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if s.publishGen() > after {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *Shell) broadcastKey(k keyEv) {
	s.mu.Lock()
	var apps []*App
	for _, a := range s.apps {
		if a != nil && a.inited && a.runner != nil {
			apps = append(apps, a)
		}
	}
	s.mu.Unlock()
	for _, a := range apps {
		a.runner.HuInput(k.name, k.pressed)
	}
}

func (s *Shell) takeKeys() []keyEv {
	s.mu.Lock()
	defer s.mu.Unlock()
	ks := s.pendingKeys
	s.pendingKeys = nil
	return ks
}

func (s *Shell) takeApps() []*App {
	s.mu.Lock()
	defer s.mu.Unlock()
	as := s.pendingApps
	s.pendingApps = nil
	return as
}

func (s *Shell) takeTouches() [][3]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.pendingTouch
	s.pendingTouch = nil
	return ts
}

func (s *Shell) takeNotes() []pendingNote {
	s.mu.Lock()
	defer s.mu.Unlock()
	ns := s.pendingNotes
	s.pendingNotes = nil
	return ns
}

func drawInitPlaceholder(dst *image.RGBA, id string) {
	if id == "" {
		id = "basic"
	}
	b := dst.Bounds()
	draw.Draw(dst, b, &image.Uniform{C: color.RGBA{A: 0xff}}, image.Point{}, draw.Src)
	livevid.DrawString(dst, "init - "+id, b.Dx()/2, b.Dy()/2, 2, color.RGBA{R: 0xe8, G: 0xee, B: 0xf4, A: 0xff}, true)
}

func drawShellError(dst *image.RGBA, id, why string) {
	if id == "" {
		id = "basic"
	}
	b := dst.Bounds()
	draw.Draw(dst, b, &image.Uniform{C: color.RGBA{A: 0xff}}, image.Point{}, draw.Src)
	livevid.DrawString(dst, "error: shell "+id+" "+why, b.Dx()/2, b.Dy()/2, 2, color.RGBA{R: 0xe8, G: 0xee, B: 0xf4, A: 0xff}, true)
}

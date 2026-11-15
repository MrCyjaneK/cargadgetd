package shell

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/cargadgetd/cargadgetd/internal/gps"
)

type fakeRunner struct {
	started bool
	hu      []string
	gps     []gpsRec
	id      string
	stopLog *[]string
}

func (f *fakeRunner) Start(play func(pcm []int16, nframes, ch, rate int)) { f.started = true }
func (f *fakeRunner) Stop() {
	if f.stopLog != nil {
		*f.stopLog = append(*f.stopLog, f.id)
	}
}
func (f *fakeRunner) HuInput(name string, pressed int) {
	f.hu = append(f.hu, name)
}
func (f *fakeRunner) GPSFix(hasPos int, lat, lon, accM float64, hasSpeed, kmh, hasCourse int, course float64, err string) {
	f.gps = append(f.gps, gpsRec{kind: "fix", hasPos: hasPos, lat: lat, lon: lon, kmh: kmh, hasSpeed: hasSpeed, err: err})
}
func (f *fakeRunner) GPSSat(gain int, kicker, title, sub, detail, flag string) {
	f.gps = append(f.gps, gpsRec{kind: "sat", gain: gain, title: title, flag: flag})
}
func (f *fakeRunner) AppPtrs() (draw, touch, pause, resume unsafe.Pointer) { return }

type gpsRec struct {
	kind             string
	hasPos, hasSpeed int
	kmh, gain        int
	lat, lon         float64
	err, title, flag string
}

func TestMapTouch(t *testing.T) {
	down, ok := MapTouch(0)
	if !ok || down != 1 {
		t.Fatalf("press: %d %v", down, ok)
	}
	up, ok := MapTouch(1)
	if !ok || up != 0 {
		t.Fatalf("release: %d %v", up, ok)
	}
	move, ok := MapTouch(2)
	if !ok || move != 2 {
		t.Fatalf("drag: %d %v", move, ok)
	}
	down, ok = MapTouch(3)
	if !ok || down != 1 {
		t.Fatalf("action 3: %d %v", down, ok)
	}
	up, ok = MapTouch(4)
	if !ok || up != 0 {
		t.Fatalf("action 4: %d %v", up, ok)
	}
}

func TestFramePeriodDefaultAndSet(t *testing.T) {
	s := New(800, 480, 48000, 2)
	if s.FramePeriod() != DefaultFramePeriod {
		t.Fatalf("default %v", s.FramePeriod())
	}
	s.SetFramePeriod(50 * time.Millisecond)
	if s.FramePeriod() != 50*time.Millisecond {
		t.Fatalf("set %v", s.FramePeriod())
	}
	s.SetFramePeriod(0)
	if s.FramePeriod() != DefaultFramePeriod {
		t.Fatalf("zero %v", s.FramePeriod())
	}
	s.SetFramePeriod(10 * time.Millisecond)
	if s.FramePeriod() != MinFramePeriod {
		t.Fatalf("floor %v", s.FramePeriod())
	}
}

func TestHuInputBroadcastsToAllApps(t *testing.T) {
	s := New(800, 480, 48000, 2)
	a := &fakeRunner{}
	b := &fakeRunner{}
	s.attach(&App{ID: "music", Name: "Music", runner: a})
	s.attach(&App{ID: "hello", Name: "Hello", runner: b})
	s.StartApps()
	s.HuInput("next", 1)
	s.HuInput("next", 0)
	s.Draw(image.NewRGBA(image.Rect(0, 0, 800, 480)), time.Now())
	if len(a.hu) != 2 || a.hu[0] != "next" || a.hu[1] != "next" {
		t.Fatalf("music %v", a.hu)
	}
	if len(b.hu) != 2 || b.hu[0] != "next" {
		t.Fatalf("hello %v", b.hu)
	}
}

func TestCloseStopsApps(t *testing.T) {
	var order []string
	s := New(800, 480, 48000, 2)
	mk := func(id string) *fakeRunner {
		return &fakeRunner{id: id, stopLog: &order}
	}
	s.attach(&App{ID: "link", Name: "Link", runner: mk("link")})
	s.attach(&App{ID: "music", Name: "Music", runner: mk("music")})
	s.attach(&App{ID: "maps", Name: "Maps", runner: mk("maps")})
	s.Close()
	if len(order) != 3 || order[0] != "link" || order[1] != "music" || order[2] != "maps" {
		t.Fatalf("stop order %v", order)
	}
}

func TestDrawPlaceholderWithoutShell(t *testing.T) {
	s := New(800, 480, 48000, 2)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{R: 0xff, A: 0xff}}, image.Point{}, draw.Src)
	s.Draw(dst, time.Now())
	if dst.RGBAAt(0, 0).R != 0 || dst.RGBAAt(0, 0).A != 0xff {
		t.Fatalf("placeholder pixel %+v", dst.RGBAAt(0, 0))
	}
}

func TestDrawErrorWhenShellMissing(t *testing.T) {
	s := New(800, 480, 48000, 2)
	if err := s.OpenShell(t.TempDir(), "neon"); err == nil {
		t.Fatal("expected missing shell")
	}
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	t0 := time.Now()
	s.Draw(dst, time.Now())
	if elapsed := time.Since(t0); elapsed > 100*time.Millisecond {
		t.Fatalf("Draw took %s", elapsed)
	}
	init := image.NewRGBA(dst.Bounds())
	drawInitPlaceholder(init, "neon")
	if bytes.Equal(dst.Pix, init.Pix) {
		t.Fatal("missing shell still looks like init")
	}
	want := image.NewRGBA(dst.Bounds())
	drawShellError(want, "neon", "did not load")
	if !bytes.Equal(dst.Pix, want.Pix) {
		t.Fatal("error frame mismatch")
	}
}

type blockSurf struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockSurf) init(int, int, int)                {}
func (b *blockSurf) pushApp(*App)                      {}
func (b *blockSurf) notif(string, string, *image.RGBA) {}
func (b *blockSurf) draw(*image.RGBA, time.Time) {
	b.once.Do(func() { close(b.started) })
	<-b.release
}
func (b *blockSurf) touch(int, int, int) {}
func (b *blockSurf) huinput(string, int) {}
func (b *blockSurf) stop()               {}

func TestDrawErrorWhenDrawHangs(t *testing.T) {
	s := New(800, 480, 48000, 2)
	b := &blockSurf{started: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	unblock := func() { once.Do(func() { close(b.release) }) }
	t.Cleanup(func() {
		unblock()
		s.Close()
	})
	s.mu.Lock()
	s.plug = b
	done := s.armDrawLocked()
	s.mu.Unlock()
	if done == nil {
		t.Fatal("draw loop was not armed")
	}
	s.ready.Store(true)
	s.SetDropFrames(1)
	go func() {
		defer close(done)
		s.drawLoop(b)
	}()

	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	t0 := time.Now()
	s.Draw(dst, time.Now())
	if elapsed := time.Since(t0); elapsed > 100*time.Millisecond {
		t.Fatalf("Draw blocked for %s", elapsed)
	}
	select {
	case <-b.started:
	case <-time.After(time.Second):
		t.Fatal("draw was not called")
	}
	t0 = time.Now()
	s.Draw(dst, time.Now())
	if elapsed := time.Since(t0); elapsed > 100*time.Millisecond {
		t.Fatalf("hung Draw blocked for %s", elapsed)
	}
	want := image.NewRGBA(dst.Bounds())
	drawShellError(want, "basic", "not responding")
	if !bytes.Equal(dst.Pix, want.Pix) {
		t.Fatal("hung draw did not paint the error")
	}
}

func TestDropFramesBeforeUnresponsive(t *testing.T) {
	s := New(800, 480, 48000, 2)
	s.plug = &blockSurf{}
	s.ready.Store(true)
	s.frameMu.Lock()
	s.drawStart = time.Now()
	s.drawEpoch = 1
	s.frameMu.Unlock()
	s.SetDropFrames(3)

	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	want := image.NewRGBA(dst.Bounds())
	drawShellError(want, "basic", "not responding")
	for i := 1; i <= 2; i++ {
		s.Draw(dst, time.Now())
		if bytes.Equal(dst.Pix, want.Pix) {
			t.Fatalf("error on dropped frame %d", i)
		}
	}
	s.Draw(dst, time.Now())
	if !bytes.Equal(dst.Pix, want.Pix) {
		t.Fatal("expected the error after 3 dropped frames")
	}

	s.frameMu.Lock()
	s.drawEpoch = 2
	s.frameMu.Unlock()
	s.Draw(dst, time.Now())
	if bytes.Equal(dst.Pix, want.Pix) {
		t.Fatal("a new draw restarted the drop count on the error line")
	}

	s.SetDropFrames(0)
	s.frameMu.Lock()
	s.drawStart = time.Now()
	s.drawEpoch = 3
	s.dropped = 0
	s.frameMu.Unlock()
	for i := 0; i < 30; i++ {
		s.Draw(dst, time.Now())
		if bytes.Equal(dst.Pix, want.Pix) {
			t.Fatal("drop_frames 0 painted the error")
		}
	}
}

func TestNormalizeShellID(t *testing.T) {
	if got := NormalizeShellID(""); got != "basic" {
		t.Fatalf("%q", got)
	}
	if got := NormalizeShellID("  basic "); got != "basic" {
		t.Fatalf("%q", got)
	}
	if got := NormalizeShellID("foo-bar"); got != "basic" {
		t.Fatalf("%q", got)
	}
	if got := NormalizeShellID("night"); got != "night" {
		t.Fatalf("%q", got)
	}
}

type closeSrc struct {
	mu     sync.Mutex
	closed bool
}

func (c *closeSrc) Snapshot() gps.Fix { return gps.Fix{HasSpeed: true, Kmh: 3} }
func (c *closeSrc) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

type recPlug struct {
	fixes int
	sats  []string
}

func (recPlug) init(int, int, int)                {}
func (recPlug) pushApp(*App)                      {}
func (recPlug) notif(string, string, *image.RGBA) {}
func (recPlug) draw(*image.RGBA, time.Time)       {}
func (recPlug) touch(int, int, int)               {}
func (recPlug) huinput(string, int)               {}
func (recPlug) stop()                             {}
func (r *recPlug) gpsFix(int, float64, float64, float64, int, int, int, float64, string) {
	r.fixes++
}
func (r *recPlug) gpsSat(_ int, _, title, _, _, _ string) {
	r.sats = append(r.sats, title)
}

func TestGPSBroadcastReplaysFixAndDropsSat(t *testing.T) {
	s := New(800, 480, 48000, 2)
	plug := &recPlug{}
	s.plug = plug
	s.ready.Store(false)
	a := &fakeRunner{}
	s.attach(&App{ID: "maps", Name: "Maps", runner: a})
	s.StartApps()
	s.deliverGPS(gps.Fix{
		HasPos: true, Lat: 50, Lon: 19, HasSpeed: true, Kmh: 40,
		Notes: []gps.SatNote{{Gain: true, Title: "G27", Flag: "us"}},
	})
	s.Draw(image.NewRGBA(image.Rect(0, 0, 8, 8)), time.Now())
	if len(a.gps) != 2 || a.gps[0].kind != "fix" || a.gps[0].kmh != 40 || a.gps[0].lat != 50 || a.gps[1].kind != "sat" || a.gps[1].title != "G27" {
		t.Fatalf("maps %v", a.gps)
	}
	if plug.fixes != 0 || len(plug.sats) != 0 {
		t.Fatalf("shell got gps before it was ready: %+v", plug)
	}
	s.ready.Store(true)
	s.Draw(image.NewRGBA(image.Rect(0, 0, 8, 8)), time.Now())
	if plug.fixes != 1 || len(plug.sats) != 1 || plug.sats[0] != "G27" {
		t.Fatalf("shell replay %+v", plug)
	}
	if len(a.gps) != 2 {
		t.Fatalf("maps saw the sat twice %v", a.gps)
	}
	b := &fakeRunner{}
	s.attach(&App{ID: "area", Name: "Area", runner: b})
	s.StartApps()
	s.Draw(image.NewRGBA(image.Rect(0, 0, 8, 8)), time.Now())
	if len(b.gps) != 1 || b.gps[0].kind != "fix" || b.gps[0].kmh != 40 {
		t.Fatalf("late app %v", b.gps)
	}
	if len(a.gps) != 2 || plug.fixes != 1 {
		t.Fatalf("sticky fix was sent again maps=%v shell=%d", a.gps, plug.fixes)
	}
}

func TestGPSCloseReleasesSource(t *testing.T) {
	s := New(800, 480, 48000, 2)
	c := &closeSrc{}
	prev := openGPS
	openGPS = func(string) gps.Source { return c }
	t.Cleanup(func() { openGPS = prev })
	s.StartGPS("mock")
	s.Close()
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if !closed {
		t.Fatal("gps source still held")
	}
}

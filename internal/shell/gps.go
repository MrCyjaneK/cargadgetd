package shell

import (
	"time"

	"github.com/cargadgetd/cargadgetd/internal/gps"
)

// openGPS is the location-client factory. Tests replace it.
var openGPS = gps.Open

// shellGPS is the optional shell-plugin half of the location broadcast.
// A shell that does not export the symbols does not implement it.
type shellGPS interface {
	gpsFix(hasPos int, lat, lon, accM float64, hasSpeed, kmh, hasCourse int, course float64, err string)
	gpsSat(gain int, kicker, title, sub, detail, flag string)
}

// StartGPS opens the session location client. One hold for the session;
// Close releases it. kind empty or unknown is geoclue.
func (s *Shell) StartGPS(kind string) {
	kind = gps.Normalize(kind)
	s.mu.Lock()
	if s.closed || s.gpsSrc != nil {
		s.mu.Unlock()
		return
	}
	src := openGPS(kind)
	stop := make(chan struct{})
	done := make(chan struct{})
	s.gpsSrc = src
	s.gpsStop = stop
	s.gpsDone = done
	s.mu.Unlock()
	go s.gpsPoll(src, stop, done)
}

func (s *Shell) gpsPoll(src gps.Source, stop <-chan struct{}, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.deliverGPS(src.Snapshot())
		}
	}
}

// deliverGPS queues a snapshot. The fix is sticky. Satellite notes are
// one-shot and are dropped after the next flush.
func (s *Shell) deliverGPS(f gps.Fix) {
	notes := append([]gps.SatNote(nil), f.Notes...)
	f.Notes = nil
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if !s.gpsHave || !sameGPS(s.gpsLast, f) {
		s.gpsLast = f
		s.gpsHave = true
		s.gpsGen++
	}
	if len(notes) > 0 {
		s.gpsNotes = append(s.gpsNotes, notes...)
		if len(s.gpsNotes) > 64 {
			s.gpsNotes = s.gpsNotes[len(s.gpsNotes)-64:]
		}
	}
}

func sameGPS(a, b gps.Fix) bool {
	return a.HasPos == b.HasPos && a.Lat == b.Lat && a.Lon == b.Lon && a.AccM == b.AccM &&
		a.HasSpeed == b.HasSpeed && a.Kmh == b.Kmh && a.HasCourse == b.HasCourse &&
		a.Course == b.Course && a.Err == b.Err
}

// flushGPS runs on the video thread, before the shell paints. Every inited
// app that exports the symbols is called, then the shell once it is ready.
// A fix is replayed to a subscriber that loaded after it arrived. A satellite
// note is delivered once.
func (s *Shell) flushGPS() {
	s.mu.Lock()
	if !s.gpsHave && len(s.gpsNotes) == 0 && len(s.gpsShell) == 0 {
		s.mu.Unlock()
		return
	}
	gen := s.gpsGen
	fix := s.gpsLast
	have := s.gpsHave
	notes := s.gpsNotes
	s.gpsNotes = nil
	ready := s.ready.Load()
	var shellNotes []gps.SatNote
	if ready {
		shellNotes = append(s.gpsShell, notes...)
		s.gpsShell = nil
	} else if len(notes) > 0 {
		s.gpsShell = append(s.gpsShell, notes...)
		if len(s.gpsShell) > 64 {
			s.gpsShell = s.gpsShell[len(s.gpsShell)-64:]
		}
	}
	shellBehind := ready && s.shellGen != gen && have
	if shellBehind {
		s.shellGen = gen
	}
	if s.gpsSent == nil {
		s.gpsSent = map[*App]int{}
	}
	type appGen struct {
		a   *App
		gen int
	}
	var due []appGen
	for _, a := range s.apps {
		if a == nil || !a.inited || a.runner == nil {
			continue
		}
		if have && s.gpsSent[a] != gen {
			s.gpsSent[a] = gen
			due = append(due, appGen{a: a, gen: gen})
		} else if len(notes) > 0 {
			due = append(due, appGen{a: a})
		}
	}
	plug := s.plug
	s.mu.Unlock()

	for _, d := range due {
		if have && d.gen == gen {
			d.a.runner.GPSFix(boolInt(fix.HasPos), fix.Lat, fix.Lon, fix.AccM, boolInt(fix.HasSpeed), fix.Kmh, boolInt(fix.HasCourse), fix.Course, fix.Err)
		}
		for _, n := range notes {
			d.a.runner.GPSSat(boolInt(n.Gain), n.Kicker, n.Title, n.Sub, n.Detail, n.Flag)
		}
	}
	g, _ := plug.(shellGPS)
	if g == nil {
		return
	}
	if s.asyncDraw.Load() {
		s.enqueueShellGPS(shellBehind, fix, shellNotes)
		return
	}
	if shellBehind {
		g.gpsFix(boolInt(fix.HasPos), fix.Lat, fix.Lon, fix.AccM, boolInt(fix.HasSpeed), fix.Kmh, boolInt(fix.HasCourse), fix.Course, fix.Err)
	}
	for _, n := range shellNotes {
		g.gpsSat(boolInt(n.Gain), n.Kicker, n.Title, n.Sub, n.Detail, n.Flag)
	}
}

type shellGPSEv struct {
	fix   bool
	pos   gps.Fix
	notes []gps.SatNote
}

func (s *Shell) enqueueShellGPS(fix bool, pos gps.Fix, notes []gps.SatNote) {
	if !fix && len(notes) == 0 {
		return
	}
	s.mu.Lock()
	s.pendingShellGPS = append(s.pendingShellGPS, shellGPSEv{fix: fix, pos: pos, notes: notes})
	if len(s.pendingShellGPS) > 8 {
		s.pendingShellGPS = s.pendingShellGPS[len(s.pendingShellGPS)-8:]
	}
	s.mu.Unlock()
}

// deliverShellGPS runs on the shell draw goroutine, under shellMu.
func (s *Shell) deliverShellGPS(plug shellSurface) {
	s.mu.Lock()
	evs := s.pendingShellGPS
	s.pendingShellGPS = nil
	s.mu.Unlock()
	g, _ := plug.(shellGPS)
	if g == nil {
		return
	}
	for _, ev := range evs {
		if ev.fix {
			f := ev.pos
			g.gpsFix(boolInt(f.HasPos), f.Lat, f.Lon, f.AccM, boolInt(f.HasSpeed), f.Kmh, boolInt(f.HasCourse), f.Course, f.Err)
		}
		for _, n := range ev.notes {
			g.gpsSat(boolInt(n.Gain), n.Kicker, n.Title, n.Sub, n.Detail, n.Flag)
		}
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

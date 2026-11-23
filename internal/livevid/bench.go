package livevid

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	benchWindow = 3 * time.Second
	benchStall  = 250 * time.Millisecond
	benchTick   = 100 * time.Millisecond
)

var BenchSizes = [][2]int{
	{800, 480},
	{1280, 720},
	{1920, 1080},
	{2560, 1440},
	{3840, 2160},
}

type Cell struct {
	W    int     `json:"w"`
	H    int     `json:"h"`
	Mode string  `json:"mode"`
	Load float64 `json:"load"`
	FPS  float64 `json:"fps"`

	Elapsed float64 `json:"elapsed"`
	Held    bool    `json:"held"`
	Late    bool    `json:"late"`
	Error   string  `json:"error"`
	Reason  string  `json:"reason"`
}

type Result struct {
	Encoder string
	Cells   []Cell
}

func CanSelect(cells []Cell) bool {
	for _, c := range cells {
		if c.Held {
			return true
		}
	}
	return false
}

type BenchOptions struct {
	Encoder  string
	Sizes    [][2]int
	Window   time.Duration
	Stall    time.Duration
	Clock    clock
	Stat     func() (idle, total uint64, err error)
	Open     func(w, h, fps, bps int, want string) (codec, error)
	OnStatus func(string)
	OnLive   func(Cell)
	OnCell   func(Cell)
}

type clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }
func (wallClock) Sleep(d time.Duration) {
	if d > 0 {
		time.Sleep(d)
	}
}

type cellMode struct {
	mode  string
	fps   int
	paced bool
	title string
}

var benchModes = []cellMode{
	{mode: "30", fps: 30, paced: true, title: "30 fps"},
	{mode: "60", fps: 60, paced: true, title: "60 fps"},
	{mode: "max", fps: 60, paced: false, title: "max"},
}

func Run(opt BenchOptions) Result {
	opt.Encoder = strings.TrimSpace(opt.Encoder)
	if opt.Clock == nil {
		opt.Clock = wallClock{}
	}
	if opt.Stat == nil {
		opt.Stat = readProcStat
	}
	if opt.Open == nil {
		opt.Open = openCodec
	}
	if opt.Window <= 0 {
		opt.Window = benchWindow
	}
	if opt.Stall <= 0 {
		opt.Stall = benchStall
	}
	sizes := opt.Sizes
	if len(sizes) == 0 {
		sizes = BenchSizes
	}
	out := Result{Encoder: opt.Encoder}
	for _, sz := range sizes {
		var frame *image.RGBA
		var strip []byte
		var stripH int
		for _, mode := range benchModes {
			cell := measureCell(opt, sz[0], sz[1], mode, &frame, &strip, &stripH)
			out.Cells = append(out.Cells, cell)
			if opt.OnCell != nil {
				opt.OnCell(cell)
			}
		}
	}
	return out
}

func measureCell(opt BenchOptions, w, h int, mode cellMode, frame **image.RGBA, strip *[]byte, stripH *int) Cell {
	cell := Cell{W: w, H: h, Mode: mode.mode}
	clk := opt.Clock
	note(opt, "Testing", w, h, mode.title)

	bps := hwBitrate(w, h, mode.fps, 400)
	enc, err := opt.Open(w, h, mode.fps, bps, opt.Encoder)
	if err != nil {
		cell.Error = err.Error()
		return cell
	}
	defer enc.Close()

	if *frame == nil || (*frame).Bounds().Dx() != w || (*frame).Bounds().Dy() != h {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		*frame = img
		*strip, *stripH = prepareFrame(img)
	}
	i0, t0, statErr := opt.Stat()
	report := func(frames int, late bool, elapsed time.Duration) {
		if opt.OnLive == nil || elapsed <= 0 || frames < 1 {
			return
		}
		partial := cell
		partial.FPS = float64(frames) / elapsed.Seconds()
		partial.Elapsed = elapsed.Seconds()
		if mode.paced && !late {
			partial.Held = true
		}
		if mode.paced && late {
			partial.Late = true
		}
		opt.OnLive(partial)
	}
	frames, late, elapsed, reason, encErr := runEncode(clk, enc, *frame, *strip, *stripH, mode, opt.Window, opt.Stall, report)
	i1, t1, statErr2 := opt.Stat()
	if statErr == nil && statErr2 == nil {
		cell.Load = cpuPercent(i0, t0, i1, t1)
	}
	if elapsed > 0 && frames > 0 {
		cell.FPS = float64(frames) / elapsed.Seconds()
		cell.Elapsed = elapsed.Seconds()
	}
	if encErr != nil {
		cell.Error = encErr.Error()
		return cell
	}
	cell.Reason = reason
	if mode.paced && frames > 0 && !late && reason == "" {
		cell.Held = true
	}
	if mode.paced && late {
		cell.Late = true
	}
	return cell
}

func note(opt BenchOptions, phase string, w, h int, title string) {
	if opt.OnStatus == nil {
		return
	}
	opt.OnStatus(fmt.Sprintf("%s - %d×%d - %s", phase, w, h, title))
}

func runEncode(clk clock, enc codec, img *image.RGBA, strip []byte, stripH int, mode cellMode, window, stall time.Duration, report func(int, bool, time.Duration)) (frames int, late bool, elapsed time.Duration, reason string, err error) {
	start := clk.Now()
	if mode.paced {
		frames, late, reason, err = runPaced(clk, enc, img, strip, stripH, mode.fps, start, window, stall, report)
	} else {
		frames, reason, err = runMax(clk, enc, img, strip, stripH, start, window, stall, report)
	}
	elapsed = clk.Now().Sub(start)
	return frames, late, elapsed, reason, err
}

func runPaced(clk clock, enc codec, img *image.RGBA, strip []byte, stripH, fps int, start time.Time, window, stall time.Duration, report func(int, bool, time.Duration)) (frames int, late bool, reason string, err error) {
	frameDur := time.Second / time.Duration(fps)
	if frameDur <= 0 {
		frameDur = time.Second / 30
	}
	deadline := start
	var last time.Time
	var empties int
	for clk.Now().Sub(start) < window {
		placeStrip(img, strip, stripH, frames)
		took, n, encErr := encodeOne(clk, enc, img)
		if encErr != nil {
			return frames, false, "", encErr
		}
		if n == 0 {
			empties++
			if empties > 2 {
				return frames, false, "", fmt.Errorf("encoder produced no picture")
			}
			continue
		}
		empties = 0
		frames++
		if took > stall {
			return frames, true, stallReason(took), nil
		}
		deadline = deadline.Add(frameDur)
		now := clk.Now()
		if now.After(deadline) {
			late = true
		} else {
			clk.Sleep(deadline.Sub(now))
		}
		last = tickLive(clk, start, last, frames, late, report)
	}
	return frames, late, "", nil
}

func runMax(clk clock, enc codec, img *image.RGBA, strip []byte, stripH int, start time.Time, window, stall time.Duration, report func(int, bool, time.Duration)) (frames int, reason string, err error) {
	var last time.Time
	var empties int
	for clk.Now().Sub(start) < window {
		placeStrip(img, strip, stripH, frames)
		took, n, encErr := encodeOne(clk, enc, img)
		if encErr != nil {
			return frames, "", encErr
		}
		if n == 0 {
			empties++
			if empties > 2 {
				return frames, "", fmt.Errorf("encoder produced no picture")
			}
			continue
		}
		empties = 0
		frames++
		if took > stall {
			return frames, stallReason(took), nil
		}
		last = tickLive(clk, start, last, frames, false, report)
	}
	return frames, "", nil
}

func tickLive(clk clock, start, last time.Time, frames int, late bool, report func(int, bool, time.Duration)) time.Time {
	if report == nil {
		return last
	}
	now := clk.Now()
	if !last.IsZero() && now.Sub(last) < benchTick {
		return last
	}
	report(frames, late, now.Sub(start))
	return now
}

func encodeOne(clk clock, enc codec, img *image.RGBA) (time.Duration, int, error) {
	t0 := clk.Now()
	au, err := enc.Encode(img)
	took := clk.Now().Sub(t0)
	if took <= 0 {
		clk.Sleep(time.Nanosecond)
		took = clk.Now().Sub(t0)
	}
	return took, len(au), err
}

func stallReason(took time.Duration) string {
	ms := took.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return fmt.Sprintf("encode took %dms", ms)
}

func cpuPercent(idle0, total0, idle1, total1 uint64) float64 {
	if total1 <= total0 {
		return 0
	}
	dt := total1 - total0
	var busy uint64
	if idle1 >= idle0 {
		di := idle1 - idle0
		if di >= dt {
			return 0
		}
		busy = dt - di
	} else {
		busy = dt
	}
	return float64(busy) * 100 / float64(dt)
}

func readProcStat() (uint64, uint64, error) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return parseCPULine(line)
}

func parseCPULine(line string) (idle, total uint64, err error) {
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("livevid: bad /proc/stat line")
	}
	for i := 1; i < len(fields); i++ {
		n, conv := strconv.ParseUint(fields[i], 10, 64)
		if conv != nil {
			return 0, 0, conv
		}
		total += n
		if i == 4 {
			idle = n
		}
	}
	return idle, total, nil
}

func prepareFrame(img *image.RGBA) (strip []byte, stripH int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return nil, 0
	}
	for y := 0; y < h; y++ {
		c := uint8((y * 255) / h)
		off := img.PixOffset(b.Min.X, b.Min.Y+y)
		row := img.Pix[off : off+w*4]
		for x := 0; x < len(row); x += 4 {
			row[x] = c
			row[x+1] = 32
			row[x+2] = 180
			row[x+3] = 255
		}
	}
	stripH = 8
	if stripH >= h {
		stripH = 1
	}
	strip = make([]byte, stripH*w*4)
	for i := 0; i < len(strip); i += 4 {
		strip[i] = 255
		strip[i+1] = 220
		strip[i+2] = 0
		strip[i+3] = 255
	}
	return strip, stripH
}

func placeStrip(img *image.RGBA, strip []byte, stripH, frame int) {
	if stripH < 1 || len(strip) == 0 {
		return
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return
	}
	if stripH > h {
		stripH = h
	}
	y0 := 0
	if h > stripH {
		y0 = frame % (h - stripH + 1)
	}
	rowBytes := w * 4
	if len(strip) < stripH*rowBytes {
		return
	}
	for i := 0; i < stripH; i++ {
		off := img.PixOffset(b.Min.X, b.Min.Y+y0+i)
		copy(img.Pix[off:off+rowBytes], strip[i*rowBytes:(i+1)*rowBytes])
	}
}

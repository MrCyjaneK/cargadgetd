package main

import (
	"fmt"
	"image"
	"os"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/livevid"
)

func main() {
	sizes := livevid.BenchSizes
	if len(os.Args) > 1 {
		sizes = nil
		for _, a := range os.Args[1:] {
			var w, h int
			if _, err := fmt.Sscanf(a, "%dx%d", &w, &h); err != nil || w < 2 || h < 2 {
				fmt.Fprintf(os.Stderr, "size must be WxH, got %s\n", a)
				os.Exit(1)
			}
			sizes = append(sizes, [2]int{w, h})
		}
	}
	names := []string{"software", "software-fast", "droidmedia"}
	if only := os.Getenv("CARGADGETD_ONLY"); only != "" {
		names = []string{only}
	}
	for _, wh := range sizes {
		for _, name := range names {
			if err := run(name, wh[0], wh[1]); err != nil {
				fmt.Printf("%s %dx%d ERROR %v\n", name, wh[0], wh[1], err)
			}
		}
	}
}

func run(name string, w, h int) error {
	var yuvN, subN, takeN time.Duration
	var phases int
	livevid.OnPhase = func(p livevid.Phase) {
		yuvN += p.YUV
		subN += p.Submit
		takeN += p.Take
		phases++
	}
	defer func() { livevid.OnPhase = nil }()

	var frame *image.RGBA
	r, err := livevid.New(livevid.Options{
		Width: w, Height: h, FPS: 60, Bitrate: 400, Encoder: name,
		Paint: func(img *image.RGBA, _ time.Time) { frame = img },
	})
	if err != nil {
		return err
	}
	defer r.Close()

	for i := 0; i < 8; i++ {
		move(frame, i)
		if _, err := r.Encode(time.Now()); err != nil {
			return err
		}
	}
	yuvN, subN, takeN, phases = 0, 0, 0, 0

	const n = 40
	var elapsed time.Duration
	var got int
	var slow time.Duration
	for i := 0; got < n && i < n+8; i++ {
		move(frame, i+8)
		t0 := time.Now()
		au, err := r.Encode(time.Now())
		d := time.Since(t0)
		if err != nil {
			return err
		}
		if len(au) == 0 {
			continue
		}
		elapsed += d
		got++
		if d > slow {
			slow = d
		}
	}
	if got == 0 {
		return fmt.Errorf("no frames")
	}
	fps := float64(got) / elapsed.Seconds()
	fmt.Printf("%s %dx%d  fps=%.1f  frame=%.1fms  slow=%.1fms  n=%d",
		name, w, h, fps, elapsed.Seconds()/float64(got)*1000, slow.Seconds()*1000, got)
	if phases > 0 {
		fmt.Printf("  yuv=%.1fms  submit=%.1fms  take=%.1fms",
			yuvN.Seconds()/float64(phases)*1000,
			subN.Seconds()/float64(phases)*1000,
			takeN.Seconds()/float64(phases)*1000)
	}
	fmt.Println()
	return nil
}

func move(img *image.RGBA, n int) {
	if img == nil {
		return
	}
	b := img.Bounds()
	row := (n * 17) % b.Dy()
	off := img.PixOffset(0, row)
	pix := img.Pix[off : off+b.Dx()*4]
	for x := 0; x < len(pix); x += 4 {
		pix[x] = byte(n + x)
		pix[x+1] = 180
		pix[x+2] = byte(x)
		pix[x+3] = 255
	}
}

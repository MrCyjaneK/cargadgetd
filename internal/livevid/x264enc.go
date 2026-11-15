package livevid

import (
	"bytes"
	"fmt"
	"image"

	x264 "github.com/gen2brain/x264-go"
)

type x264Enc struct {
	enc *x264.Encoder
	buf bytes.Buffer
}

func newX264(w, h, fps int) (codec, error) {
	e := &x264Enc{}
	enc, err := x264.NewEncoder(&e.buf, &x264.Options{
		Width:        w,
		Height:       h,
		FrameRate:    fps,
		Tune:         "zerolatency",
		Preset:       "ultrafast",
		Profile:      "baseline",
		KeyInt:       fps * 3600,
		IntraRefresh: false,
		RateControl:  "cqp",
		RateConstant: 18, // larger loop P (~300–500B); tiny 94B+PCM died ~20s on Xperia
		LogLevel:     x264.LogError,
	})
	if err != nil {
		return nil, err
	}
	e.buf.Reset()
	e.enc = enc
	return e, nil
}

func (e *x264Enc) Name() string { return "x264-sw" }

func (e *x264Enc) Encode(img *image.RGBA) ([]byte, error) {
	e.buf.Reset()
	if err := e.enc.Encode(img); err != nil {
		return nil, err
	}
	b := e.buf.Bytes()
	if len(b) == 0 {
		return nil, fmt.Errorf("livevid: empty AU")
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func (e *x264Enc) Close() error {
	if e.enc == nil {
		return nil
	}
	err := e.enc.Close()
	e.enc = nil
	return err
}

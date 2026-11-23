package livevid

import (
	"bytes"
	"fmt"
	"image"

	x264 "github.com/gen2brain/x264-go"
)

type x264Enc struct {
	name string
	enc  *x264.Encoder
	buf  bytes.Buffer
}

func newX264(w, h, fps int) (codec, error) {
	return openX264(w, h, fps, "x264-sw", x264.Options{
		Tune:         "zerolatency",
		Preset:       "ultrafast",
		Profile:      "baseline",
		RateControl:  "cqp",
		RateConstant: 18, // larger loop P (~300–500B); tiny 94B+PCM died ~20s on Xperia
	})
}

func newX264Fast(w, h, fps int) (codec, error) {
	return openX264(w, h, fps, "x264-sw-fast", x264.Options{
		Tune:         "fastdecode,zerolatency",
		Preset:       "ultrafast",
		Profile:      "baseline",
		RateControl:  "cqp",
		RateConstant: 51,
	})
}

func openX264(w, h, fps int, name string, opts x264.Options) (codec, error) {
	e := &x264Enc{name: name}
	opts.Width = w
	opts.Height = h
	opts.FrameRate = fps
	opts.KeyInt = fps * 3600
	opts.IntraRefresh = false
	opts.LogLevel = x264.LogError
	enc, err := x264.NewEncoder(&e.buf, &opts)
	if err != nil {
		return nil, err
	}
	e.buf.Reset()
	e.enc = enc
	return e, nil
}

func (e *x264Enc) Name() string { return e.name }

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

package livevid

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestToAnnexBLengthPrefixed(t *testing.T) {
	nal := []byte{0x65, 0x88, 0x00}
	in := []byte{0, 0, 0, 3, 0x65, 0x88, 0x00}
	out := toAnnexB(in, nil, false)
	want := []byte{0, 0, 0, 1, 0x65, 0x88, 0x00}
	if !bytes.Equal(out, want) {
		t.Fatalf("got %x want %x", out, want)
	}
	sps := []byte{0, 0, 0, 1, 0x67, 0x42}
	with := toAnnexB(in, sps, true)
	if !bytes.HasPrefix(with, sps) {
		t.Fatalf("missing sps prefix: %x", with)
	}
	already := []byte{0, 0, 0, 1, 0x67, 0x42, 0, 0, 0, 1, 0x65}
	if !bytes.Equal(toAnnexB(already, sps, true), already) {
		t.Fatal("should not double-prepend sps")
	}
	_ = nal
}

func TestAvcCToAnnexB(t *testing.T) {
	// avcC: version=1, baseline, lengthSize=4, one SPS (0x67), one PPS (0x68).
	sps := []byte{0x67, 0x42, 0xc0, 0x1e}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	avcc := []byte{0x01, 0x42, 0xc0, 0x1e, 0xff, 0xe1, 0x00, byte(len(sps))}
	avcc = append(avcc, sps...)
	avcc = append(avcc, 0x01, 0x00, byte(len(pps)))
	avcc = append(avcc, pps...)
	cfg := codecConfigToAnnexB(avcc)
	if !hasSPS(cfg) {
		t.Fatalf("avcC SPS missing: %x", cfg)
	}
	idr := []byte{0, 0, 0, 4, 0x65, 0x88, 0x00, 0x00}
	out := toAnnexB(idr, avcc, true)
	if !hasSPS(out) {
		t.Fatalf("IDR missing SPS from avcC: %x", out)
	}
	if !bytes.Contains(out, []byte{0, 0, 0, 1, 0x65}) {
		t.Fatalf("IDR NAL missing: %x", out)
	}
}

func TestParameterSetsOnly(t *testing.T) {
	sps := []byte{0, 0, 0, 1, 0x67, 0x42, 0xc0, 0x1e}
	pps := []byte{0, 0, 0, 1, 0x68, 0xce, 0x38, 0x80}
	idr := []byte{0, 0, 0, 1, 0x65, 0x88, 0x00}
	if !parameterSetsOnly(append(append([]byte{}, sps...), pps...)) {
		t.Fatal("SPS+PPS should be config-only")
	}
	if parameterSetsOnly(append(append(append([]byte{}, sps...), pps...), idr...)) {
		t.Fatal("SPS+PPS+IDR is a picture")
	}
	if parameterSetsOnly(idr) {
		t.Fatal("IDR is not config")
	}
	avcc := []byte{0x01, 0x42, 0xc0, 0x1e, 0xff, 0xe1, 0x00, 4, 0x67, 0x42, 0xc0, 0x1e, 0x01, 0x00, 4, 0x68, 0xce, 0x38, 0x80}
	if !parameterSetsOnly(avcc) {
		t.Fatal("avcC should be config-only")
	}
}

func TestToAnnexBPrependWithoutSyncFlag(t *testing.T) {
	sps := []byte{0, 0, 0, 1, 0x67, 0x42}
	idr := []byte{0, 0, 0, 4, 0x65, 0x88, 0x00, 0x00}
	out := toAnnexB(idr, sps, false)
	if !hasSPS(out) {
		t.Fatalf("IDR without sync flag still needs SPS: %x", out)
	}
}

func TestRGBAToNV12(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	dst := make([]byte, yuvSize(4, 4))
	rgbaToYUV(dst, img, yuvNV12)
	if dst[0] < 70 {
		t.Fatalf("red Y too dark: %d", dst[0])
	}
	// NV12 chroma after Y plane (16 bytes)
	if len(dst) != 24 {
		t.Fatalf("size %d", len(dst))
	}
}

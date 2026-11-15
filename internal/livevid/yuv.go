package livevid

import "image"

const (
	yuvNV12 = 1
	yuvI420 = 2
	yuvYV12 = 3
)

func yuvSize(w, h int) int {
	return w*h + w*h/2
}

func rgbaToYUV(dst []byte, src *image.RGBA, layout int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	yp := dst[:w*h]
	chroma := dst[w*h:]
	var uPlane, vPlane []byte
	nv12 := layout == yuvNV12
	if !nv12 {
		cs := w * h / 4
		if layout == yuvI420 {
			uPlane, vPlane = chroma[:cs], chroma[cs:]
		} else {
			vPlane, uPlane = chroma[:cs], chroma[cs:]
		}
	}
	pix := src.Pix
	stride := src.Stride
	for y := 0; y < h; y++ {
		row := pix[y*stride:]
		for x := 0; x < w; x++ {
			o := x * 4
			r, g, bl := int(row[o]), int(row[o+1]), int(row[o+2])
			yp[y*w+x] = byte((77*r + 150*g + 29*bl) >> 8)
			if y&1 == 0 && x&1 == 0 {
				u := byte(((-43*r - 85*g + 128*bl) >> 8) + 128)
				v := byte(((128*r - 107*g - 21*bl) >> 8) + 128)
				cx, cy := x/2, y/2
				if nv12 {
					i := cy*(w) + cx*2
					chroma[i] = u
					chroma[i+1] = v
				} else {
					i := cy*(w/2) + cx
					uPlane[i] = u
					vPlane[i] = v
				}
			}
		}
	}
}

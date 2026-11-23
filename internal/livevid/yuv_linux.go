//go:build linux

package livevid

/*
#cgo CFLAGS: -O3
#include <stdint.h>
#if defined(__aarch64__)
#include <arm_neon.h>
#endif

static void rgba_row_y(uint8_t *yrow, const uint8_t *row, int w) {
	for (int x = 0; x < w; x++) {
		int r = row[0], g = row[1], b = row[2];
		yrow[x] = (uint8_t)((77 * r + 150 * g + 29 * b) >> 8);
		row += 4;
	}
}

static void rgba_row_uv(uint8_t *uvrow, const uint8_t *row, int w) {
	for (int x = 0; x < w; x += 2) {
		int r = row[0], g = row[1], b = row[2];
		uvrow[0] = (uint8_t)(((-43 * r - 85 * g + 128 * b) >> 8) + 128);
		uvrow[1] = (uint8_t)(((128 * r - 107 * g - 21 * b) >> 8) + 128);
		uvrow += 2;
		row += 8;
	}
}

#if defined(__aarch64__)
static void rgba_to_nv12_neon(uint8_t *dst, const uint8_t *src, int w, int h, int stride) {
	uint8_t *yp = dst;
	uint8_t *uv = dst + (size_t)w * (size_t)h;
	int w16 = w & ~15;
	for (int y = 0; y < h; y++) {
		const uint8_t *row = src + (size_t)y * (size_t)stride;
		uint8_t *yrow = yp + (size_t)y * (size_t)w;
		int x = 0;
		if ((y & 1) == 0) {
			uint8_t *uvrow = uv + (size_t)(y / 2) * (size_t)w;
			for (; x < w16; x += 16) {
				uint8x16x4_t p = vld4q_u8(row + (size_t)x * 4);
				uint16x8_t yl = vmulq_n_u16(vmovl_u8(vget_low_u8(p.val[0])), 77);
				yl = vmlaq_n_u16(yl, vmovl_u8(vget_low_u8(p.val[1])), 150);
				yl = vmlaq_n_u16(yl, vmovl_u8(vget_low_u8(p.val[2])), 29);
				uint16x8_t yh = vmulq_n_u16(vmovl_u8(vget_high_u8(p.val[0])), 77);
				yh = vmlaq_n_u16(yh, vmovl_u8(vget_high_u8(p.val[1])), 150);
				yh = vmlaq_n_u16(yh, vmovl_u8(vget_high_u8(p.val[2])), 29);
				vst1q_u8(yrow + x, vcombine_u8(vmovn_u16(vshrq_n_u16(yl, 8)), vmovn_u16(vshrq_n_u16(yh, 8))));

				uint8x8x2_t rz = vuzp_u8(vget_low_u8(p.val[0]), vget_high_u8(p.val[0]));
				uint8x8x2_t gz = vuzp_u8(vget_low_u8(p.val[1]), vget_high_u8(p.val[1]));
				uint8x8x2_t bz = vuzp_u8(vget_low_u8(p.val[2]), vget_high_u8(p.val[2]));
				int16x8_t rs = vreinterpretq_s16_u16(vmovl_u8(rz.val[0]));
				int16x8_t gs = vreinterpretq_s16_u16(vmovl_u8(gz.val[0]));
				int16x8_t bs = vreinterpretq_s16_u16(vmovl_u8(bz.val[0]));
				int16x8_t u = vmulq_n_s16(rs, -43);
				u = vmlaq_n_s16(u, gs, -85);
				u = vmlaq_n_s16(u, bs, 128);
				u = vaddq_s16(vshrq_n_s16(u, 8), vdupq_n_s16(128));
				int16x8_t v = vmulq_n_s16(rs, 128);
				v = vmlaq_n_s16(v, gs, -107);
				v = vmlaq_n_s16(v, bs, -21);
				v = vaddq_s16(vshrq_n_s16(v, 8), vdupq_n_s16(128));
				uint8x8x2_t chroma;
				chroma.val[0] = vqmovun_s16(u);
				chroma.val[1] = vqmovun_s16(v);
				vst2_u8(uvrow + x, chroma);
			}
			if (x < w) {
				rgba_row_y(yrow + x, row + (size_t)x * 4, w - x);
				rgba_row_uv(uvrow + x, row + (size_t)x * 4, w - x);
			}
		} else {
			for (; x < w16; x += 16) {
				uint8x16x4_t p = vld4q_u8(row + (size_t)x * 4);
				uint16x8_t yl = vmulq_n_u16(vmovl_u8(vget_low_u8(p.val[0])), 77);
				yl = vmlaq_n_u16(yl, vmovl_u8(vget_low_u8(p.val[1])), 150);
				yl = vmlaq_n_u16(yl, vmovl_u8(vget_low_u8(p.val[2])), 29);
				uint16x8_t yh = vmulq_n_u16(vmovl_u8(vget_high_u8(p.val[0])), 77);
				yh = vmlaq_n_u16(yh, vmovl_u8(vget_high_u8(p.val[1])), 150);
				yh = vmlaq_n_u16(yh, vmovl_u8(vget_high_u8(p.val[2])), 29);
				vst1q_u8(yrow + x, vcombine_u8(vmovn_u16(vshrq_n_u16(yl, 8)), vmovn_u16(vshrq_n_u16(yh, 8))));
			}
			if (x < w)
				rgba_row_y(yrow + x, row + (size_t)x * 4, w - x);
		}
	}
}
#endif

static void rgba_to_nv12(uint8_t *dst, const uint8_t *src, int w, int h, int stride) {
#if defined(__aarch64__)
	rgba_to_nv12_neon(dst, src, w, h, stride);
#else
	uint8_t *yp = dst;
	uint8_t *uv = dst + (size_t)w * (size_t)h;
	for (int y = 0; y < h; y++) {
		const uint8_t *row = src + (size_t)y * (size_t)stride;
		rgba_row_y(yp + (size_t)y * (size_t)w, row, w);
		if ((y & 1) == 0)
			rgba_row_uv(uv + (size_t)(y / 2) * (size_t)w, row, w);
	}
#endif
}
*/
import "C"
import (
	"image"
	"unsafe"
)

func init() {
	rgbaToNV12 = rgbaToNV12C
}

func rgbaToNV12C(dst []byte, src *image.RGBA) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || len(src.Pix) == 0 || len(dst) < yuvSize(w, h) {
		return
	}
	C.rgba_to_nv12(
		(*C.uint8_t)(unsafe.Pointer(&dst[0])),
		(*C.uint8_t)(unsafe.Pointer(&src.Pix[0])),
		C.int(w), C.int(h), C.int(src.Stride),
	)
}

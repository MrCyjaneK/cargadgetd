//go:build linux && cgo

package shell

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo LDFLAGS: -ldl
#include "cg_shell.h"
#include <stdint.h>
#include <stdlib.h>

struct cg_shell_syms;

void *cg_shell_dlopen(const char *path, char **err);
struct cg_shell_syms *cg_shell_load(void *h, const char *id, char **err);
void cg_shell_init(struct cg_shell_syms *s, int frame_ms, int w, int h);
void cg_shell_app(struct cg_shell_syms *s, const char *id, const char *name,
	const uint8_t *icon, int iw, int ih, int istride,
	void *draw, void *touch, void *pause, void *resume);
void cg_shell_notif(struct cg_shell_syms *s, const char *title, const char *body,
	const uint8_t *icon, int iw, int ih, int istride);
void cg_shell_draw(struct cg_shell_syms *s, uint32_t *rgba, int w, int h, int stride, int64_t now_ms);
void cg_shell_touch(struct cg_shell_syms *s, int x, int y, int action);
void cg_shell_huinput(struct cg_shell_syms *s, const char *event, int pressed);
void cg_shell_gps_fix(struct cg_shell_syms *s, int has_pos, double lat, double lon, double acc_m,
	int has_speed, int kmh, int has_course, double course, const char *err);
void cg_shell_gps_sat(struct cg_shell_syms *s, int gain, const char *kicker, const char *title,
	const char *sub, const char *detail, const char *flag);
void cg_shell_stop(struct cg_shell_syms *s);
*/
import "C"

import (
	"fmt"
	"image"
	"time"
	"unsafe"
)

type shellPlug struct {
	id   string
	lib  unsafe.Pointer
	syms *C.struct_cg_shell_syms
}

func openShellPlugin(path, id string) (shellSurface, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	h := C.cg_shell_dlopen(cpath, &cerr)
	if h == nil {
		msg := "dlopen failed"
		if cerr != nil {
			msg = C.GoString(cerr)
		}
		return nil, fmt.Errorf("%s: %s", path, msg)
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	cerr = nil
	syms := C.cg_shell_load(h, cid, &cerr)
	if syms == nil {
		msg := "missing shell symbols"
		if cerr != nil {
			msg = C.GoString(cerr)
		}
		return nil, fmt.Errorf("%s: %s", path, msg)
	}
	return &shellPlug{id: id, lib: h, syms: syms}, nil
}

func (p *shellPlug) init(frameMS, w, h int) {
	C.cg_shell_init(p.syms, C.int(frameMS), C.int(w), C.int(h))
}

func (p *shellPlug) pushApp(a *App) {
	if p == nil || a == nil || a.runner == nil {
		return
	}
	draw, touch, pause, resume := a.runner.AppPtrs()
	if draw == nil {
		return
	}
	cid := C.CString(a.ID)
	cname := C.CString(a.Name)
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(cname))
	var pix *C.uint8_t
	var iw, ih, stride C.int
	if a.Icon != nil && len(a.Icon.Pix) > 0 {
		b := a.Icon.Bounds()
		pix = (*C.uint8_t)(unsafe.Pointer(&a.Icon.Pix[0]))
		iw = C.int(b.Dx())
		ih = C.int(b.Dy())
		stride = C.int(a.Icon.Stride)
	}
	C.cg_shell_app(p.syms, cid, cname, pix, iw, ih, stride, draw, touch, pause, resume)
}

func (p *shellPlug) notif(title, body string, icon *image.RGBA) {
	ct := C.CString(title)
	cb := C.CString(body)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cb))
	var pix *C.uint8_t
	var iw, ih, stride C.int
	if icon != nil && len(icon.Pix) > 0 {
		b := icon.Bounds()
		pix = (*C.uint8_t)(unsafe.Pointer(&icon.Pix[0]))
		iw = C.int(b.Dx())
		ih = C.int(b.Dy())
		stride = C.int(icon.Stride)
	}
	C.cg_shell_notif(p.syms, ct, cb, pix, iw, ih, stride)
}

func (p *shellPlug) draw(dst *image.RGBA, now time.Time) {
	if dst == nil || len(dst.Pix) == 0 {
		return
	}
	b := dst.Bounds()
	C.cg_shell_draw(p.syms, (*C.uint32_t)(unsafe.Pointer(&dst.Pix[0])), C.int(b.Dx()), C.int(b.Dy()), C.int(dst.Stride), C.int64_t(now.UnixMilli()))
}

func (p *shellPlug) touch(x, y, action int) {
	C.cg_shell_touch(p.syms, C.int(x), C.int(y), C.int(action))
}

func (p *shellPlug) huinput(event string, pressed int) {
	if event == "" {
		return
	}
	cev := C.CString(event)
	defer C.free(unsafe.Pointer(cev))
	C.cg_shell_huinput(p.syms, cev, C.int(pressed))
}

func (p *shellPlug) stop() {
	C.cg_shell_stop(p.syms)
}

func (p *shellPlug) gpsFix(hasPos int, lat, lon, accM float64, hasSpeed, kmh, hasCourse int, course float64, errMsg string) {
	var cerr *C.char
	if errMsg != "" {
		cerr = C.CString(errMsg)
		defer C.free(unsafe.Pointer(cerr))
	}
	C.cg_shell_gps_fix(p.syms, C.int(hasPos), C.double(lat), C.double(lon), C.double(accM), C.int(hasSpeed), C.int(kmh), C.int(hasCourse), C.double(course), cerr)
}

func (p *shellPlug) gpsSat(gain int, kicker, title, sub, detail, flag string) {
	ck, ct, cs, cd, cf := C.CString(kicker), C.CString(title), C.CString(sub), C.CString(detail), C.CString(flag)
	defer C.free(unsafe.Pointer(ck))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cd))
	defer C.free(unsafe.Pointer(cf))
	C.cg_shell_gps_sat(p.syms, C.int(gain), ck, ct, cs, cd, cf)
}

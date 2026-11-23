//go:build linux && cgo

package shell

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo LDFLAGS: -ldl
#include "cg_app.h"
#include <stdint.h>
#include <stdlib.h>

struct cg_syms;

void *cg_dlopen(const char *path, char **err);
struct cg_syms *cg_load_syms(void *h, const char *app, char **err);
void cg_syms_free(struct cg_syms *s);
struct cargadgetd_host *cg_host_get(void);
void cg_host_set_frame_ms(int ms);
void cg_call_init(struct cg_syms *s, const struct cargadgetd_host *h);
void cg_call_stop(struct cg_syms *s);
const char *cg_sym_name(struct cg_syms *s);
const uint8_t *cg_sym_icon(struct cg_syms *s, int *w, int *h);
void *cg_huinput_sym(void *h, const char *app, const char *event);
void cg_call_huinput_fn(void *fn, int pressed);
void *cg_nowplaying_sym(void *h, const char *app);
int cg_call_nowplaying(void *fn, struct cargadgetd_nowplaying *out);
void *cg_gps_fix_sym(void *h, const char *app);
void *cg_gps_sat_sym(void *h, const char *app);
void cg_call_gps_fix(void *fn, int has_pos, double lat, double lon, double acc_m,
	int has_speed, int kmh, int has_course, double course, const char *err);
void cg_call_gps_sat(void *fn, int gain, const char *kicker, const char *title,
	const char *sub, const char *detail, const char *flag);
void *cg_fn_draw(struct cg_syms *s);
void *cg_fn_touch(struct cg_syms *s);
void *cg_fn_pause(struct cg_syms *s);
void *cg_fn_resume(struct cg_syms *s);
*/
import "C"

import (
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"
)

var (
	playMu sync.Mutex
	playFn func(pcm []int16, nframes, ch, rate int)
)

func setHostPlay(fn func(pcm []int16, nframes, ch, rate int)) {
	playMu.Lock()
	playFn = fn
	playMu.Unlock()
}

func setHostFrameMs(ms int) {
	C.cg_host_set_frame_ms(C.int(ms))
}

func hostFrameMs() int {
	h := C.cg_host_get()
	if h == nil {
		return int(DefaultFramePeriod / time.Millisecond)
	}
	return int(h.frame_ms)
}

//export cgGoPlay
func cgGoPlay(pcm *C.int16_t, nframes, channels, sampleRate C.int) {
	if pcm == nil || nframes <= 0 || channels < 1 {
		return
	}
	n := int(nframes) * int(channels)
	src := unsafe.Slice((*int16)(unsafe.Pointer(pcm)), n)
	cp := make([]int16, n)
	copy(cp, src)
	playMu.Lock()
	fn := playFn
	playMu.Unlock()
	if fn != nil {
		fn(cp, int(nframes), int(channels), int(sampleRate))
	}
}

type soRunner struct {
	path string
	id   string
	lib  unsafe.Pointer
	syms *C.struct_cg_syms

	mu   sync.Mutex
	huFn map[string]unsafe.Pointer

	gpsLook          bool
	gpsFixFn, gpsSat unsafe.Pointer

	npLook bool
	npFn   unsafe.Pointer
}

var (
	settingsLibs   []unsafe.Pointer
	settingsLibsMu sync.Mutex
)

// openSettings maps a settings-only library for the process lifetime.
// The settings app resolves symbols with dlsym(RTLD_DEFAULT) and does not
// dlopen the file itself.
func openSettings(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	h := C.cg_dlopen(cpath, &cerr)
	if h == nil {
		msg := "dlopen failed"
		if cerr != nil {
			msg = C.GoString(cerr)
		}
		return fmt.Errorf("%s: %s", path, msg)
	}
	settingsLibsMu.Lock()
	settingsLibs = append(settingsLibs, unsafe.Pointer(h))
	settingsLibsMu.Unlock()
	return nil
}

// OpenPlugin dlopens a C ABI .so and dlsyms cargadgetd_app_<id>_*. Not plugin.Open.
func OpenPlugin(path string) (*App, error) {
	id, err := AppIDFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	h := C.cg_dlopen(cpath, &cerr)
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
	syms := C.cg_load_syms(h, cid, &cerr)
	if syms == nil {
		msg := "missing cargadgetd_app_" + id + "_draw"
		if cerr != nil {
			msg = C.GoString(cerr)
		}
		return nil, fmt.Errorf("%s: %s", path, msg)
	}

	name := C.GoString(C.cg_sym_name(syms))
	if name == "" {
		name = id
	}
	r := &soRunner{path: path, id: id, lib: h, syms: syms}
	a := &App{ID: id, Name: name, runner: r}
	var iw, ih C.int
	if p := C.cg_sym_icon(syms, &iw, &ih); p != nil && iw > 0 && ih > 0 {
		img := image.NewRGBA(image.Rect(0, 0, int(iw), int(ih)))
		n := int(iw) * int(ih) * 4
		copy(img.Pix, unsafe.Slice((*byte)(unsafe.Pointer(p)), n))
		a.Icon = img
	}
	return a, nil
}

func (r *soRunner) Start(play func(pcm []int16, nframes, ch, rate int)) {
	if play != nil {
		setHostPlay(play)
	}
	C.cg_call_init(r.syms, C.cg_host_get())
}

func (r *soRunner) Stop() {
	if r.syms != nil {
		C.cg_call_stop(r.syms)
	}
}

func (r *soRunner) HuInput(name string, pressed int) {
	r.callHuinput(name, pressed)
}

func (r *soRunner) NowPlaying() (NowPlaying, bool) {
	fn := r.nowPlayingFn()
	if fn == nil {
		return NowPlaying{}, false
	}
	var out C.struct_cargadgetd_nowplaying
	if C.cg_call_nowplaying(fn, &out) == 0 {
		return NowPlaying{}, false
	}
	np := NowPlaying{
		Title:    C.GoString(out.title),
		Artist:   C.GoString(out.artist),
		Album:    C.GoString(out.album),
		App:      C.GoString(out.app),
		State:    int(out.state),
		Duration: int(out.duration_seconds),
		Position: int(out.position_seconds),
	}
	if out.art_png != nil && out.art_len > 0 {
		np.ArtPNG = C.GoBytes(unsafe.Pointer(out.art_png), out.art_len)
	}
	return np, true
}

func (r *soRunner) nowPlayingFn() unsafe.Pointer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.npLook {
		r.npLook = true
		capp := C.CString(r.id)
		r.npFn = C.cg_nowplaying_sym(r.lib, capp)
		C.free(unsafe.Pointer(capp))
	}
	return r.npFn
}

func (r *soRunner) GPSFix(hasPos int, lat, lon, accM float64, hasSpeed, kmh, hasCourse int, course float64, errMsg string) {
	fn := r.gpsFn(true)
	if fn == nil {
		return
	}
	var cerr *C.char
	if errMsg != "" {
		cerr = C.CString(errMsg)
		defer C.free(unsafe.Pointer(cerr))
	}
	C.cg_call_gps_fix(fn, C.int(hasPos), C.double(lat), C.double(lon), C.double(accM), C.int(hasSpeed), C.int(kmh), C.int(hasCourse), C.double(course), cerr)
}

func (r *soRunner) GPSSat(gain int, kicker, title, sub, detail, flag string) {
	fn := r.gpsFn(false)
	if fn == nil {
		return
	}
	ck, ct, cs, cd, cf := C.CString(kicker), C.CString(title), C.CString(sub), C.CString(detail), C.CString(flag)
	defer C.free(unsafe.Pointer(ck))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cd))
	defer C.free(unsafe.Pointer(cf))
	C.cg_call_gps_sat(fn, C.int(gain), ck, ct, cs, cd, cf)
}

func (r *soRunner) gpsFn(fix bool) unsafe.Pointer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.gpsLook {
		r.gpsLook = true
		capp := C.CString(r.id)
		r.gpsFixFn = C.cg_gps_fix_sym(r.lib, capp)
		r.gpsSat = C.cg_gps_sat_sym(r.lib, capp)
		C.free(unsafe.Pointer(capp))
	}
	if fix {
		return r.gpsFixFn
	}
	return r.gpsSat
}

func (r *soRunner) AppPtrs() (draw, touch, pause, resume unsafe.Pointer) {
	if r.syms == nil {
		return
	}
	return C.cg_fn_draw(r.syms), C.cg_fn_touch(r.syms), C.cg_fn_pause(r.syms), C.cg_fn_resume(r.syms)
}

func (r *soRunner) callHuinput(name string, pressed int) {
	if name == "" {
		return
	}
	r.mu.Lock()
	if r.huFn == nil {
		r.huFn = make(map[string]unsafe.Pointer)
	}
	fn, cached := r.huFn[name]
	if !cached {
		capp := C.CString(r.id)
		cev := C.CString(name)
		fn = C.cg_huinput_sym(r.lib, capp, cev)
		C.free(unsafe.Pointer(capp))
		C.free(unsafe.Pointer(cev))
		r.huFn[name] = fn
	}
	r.mu.Unlock()
	if fn != nil {
		C.cg_call_huinput_fn(fn, C.int(pressed))
	}
}

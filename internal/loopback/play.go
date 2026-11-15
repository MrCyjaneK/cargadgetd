package loopback

/*
#cgo LDFLAGS: -lpthread
#include <stdlib.h>
#include <string.h>
void loopback_capture_start(int fd);
void loopback_capture_stop(void);
void loopback_set_pcm_dump(const char *prefix);
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"
)

var (
	playMu   sync.Mutex
	playFn   func(pcm []int16, nframes, channels, rate int)
	dumpPref string
)

func setCapturePlay(fn func(pcm []int16, nframes, channels, rate int)) {
	playMu.Lock()
	playFn = fn
	playMu.Unlock()
}

func setCaptureDump(prefix string) {
	playMu.Lock()
	dumpPref = strings.TrimSpace(prefix)
	playMu.Unlock()
}

func startPCMCapture(fd int) {
	if fd < 0 {
		return
	}
	playMu.Lock()
	prefix := dumpPref
	playMu.Unlock()
	if prefix != "" {
		cs := C.CString(prefix)
		C.loopback_set_pcm_dump(cs)
		C.free(unsafe.Pointer(cs))
	} else {
		C.loopback_set_pcm_dump(nil)
	}
	C.loopback_capture_start(C.int(fd))
}

func stopPCMCapture() { C.loopback_capture_stop() }

//export cgLoopbackPlay
func cgLoopbackPlay(pcm *C.int16_t, nframes, channels, rate C.int) {
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
		fn(cp, int(nframes), int(channels), int(rate))
	}
}

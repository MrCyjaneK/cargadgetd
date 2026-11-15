//go:build linux

package livevid

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// withoutAsyncPreempt runs fn on a locked OS thread with SIGURG blocked.
//
// The droidmedia worker is created from this mask, so Codec2 threads inherit
// it. MTK Codec2 does not tolerate Go's preemption signal. The same block is
// what FunctionFS uses for DWC3.
func withoutAsyncPreempt(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var set, old unix.Sigset_t
	sigAddSet(&set, int(unix.SIGURG))
	if err := unix.PthreadSigmask(unix.SIG_BLOCK, &set, &old); err != nil {
		fn()
		return
	}
	defer func() { _ = unix.PthreadSigmask(unix.SIG_SETMASK, &old, nil) }()
	fn()
}

func sigAddSet(set *unix.Sigset_t, sig int) {
	if sig <= 0 {
		return
	}
	word := (sig - 1) / 64
	bit := uint((sig - 1) % 64)
	if word >= len(set.Val) {
		return
	}
	set.Val[word] |= 1 << bit
}

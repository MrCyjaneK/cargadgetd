//go:build linux

package ffs

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// WithoutAsyncPreempt runs fn on a locked OS thread with SIGURG blocked.
//
// Go 1.14+ async preemption delivers SIGURG to interrupt goroutines stuck in
// long syscalls. FunctionFS's sync path uses wait_for_completion_interruptible;
// on EINTR it dequeues the in-flight URB. Go's os.File then retries the I/O
// (internal/poll ignoringEINTRIO). On DWC3 that requeue can land before the
// Endpoint Command Complete for EndXfer, wedging the bulk endpoint (host IN
// timeout, no USB_STATE change). Blocking SIGURG for the FFS syscall prevents
// the race without process-wide GODEBUG=asyncpreemptoff=1.
func WithoutAsyncPreempt(fn func()) {
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

// sigAddSet mirrors glibc __sigaddset: signal N is bit (N-1) in Sigset_t.
// golang.org/x/sys v0.22 has PthreadSigmask but not Sigaddset.
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

//go:build linux

package livevid

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestWithoutAsyncPreemptBlocksSIGURG(t *testing.T) {
	var blocked bool
	withoutAsyncPreempt(func() {
		var cur unix.Sigset_t
		if err := unix.PthreadSigmask(unix.SIG_SETMASK, nil, &cur); err != nil {
			t.Fatalf("PthreadSigmask: %v", err)
		}
		blocked = sigIsMember(&cur, int(unix.SIGURG))
	})
	if !blocked {
		t.Fatal("SIGURG not blocked inside withoutAsyncPreempt")
	}
	var after unix.Sigset_t
	if err := unix.PthreadSigmask(unix.SIG_SETMASK, nil, &after); err != nil {
		t.Fatalf("PthreadSigmask after: %v", err)
	}
	if sigIsMember(&after, int(unix.SIGURG)) {
		t.Fatal("SIGURG still blocked after withoutAsyncPreempt returned")
	}
}

func sigIsMember(set *unix.Sigset_t, sig int) bool {
	if sig <= 0 {
		return false
	}
	word := (sig - 1) / 64
	bit := uint((sig - 1) % 64)
	if word >= len(set.Val) {
		return false
	}
	return set.Val[word]&(1<<bit) != 0
}

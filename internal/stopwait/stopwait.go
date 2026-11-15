// Package stopwait sleeps until a deadline or a stop signal.
package stopwait

import "time"

// Sleep waits for d. It returns true when stop is already closed or closes
// during the wait. d <= 0 does not wait; an already-closed stop still
// returns true.
func Sleep(stop <-chan struct{}, d time.Duration) bool {
	if stop != nil {
		select {
		case <-stop:
			return true
		default:
		}
	}
	if d <= 0 {
		return false
	}
	if stop == nil {
		time.Sleep(d)
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return true
	case <-t.C:
		return false
	}
}

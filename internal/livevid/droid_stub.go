//go:build !linux

package livevid

import "fmt"

func newDroid(w, h, fps, bps int) (codec, error) {
	return nil, fmt.Errorf("droidmedia: not linux")
}

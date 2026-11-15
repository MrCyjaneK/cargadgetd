//go:build !linux || !cgo

package shell

import "fmt"

func setHostPlay(fn func(pcm []int16, nframes, ch, rate int)) {}

func setHostFrameMs(ms int) {}

func openSettings(path string) error {
	return fmt.Errorf("plugins: dlopen %s: not supported on this platform", path)
}

func OpenPlugin(path string) (*App, error) {
	return nil, fmt.Errorf("plugins: dlopen %s: not supported on this platform", path)
}

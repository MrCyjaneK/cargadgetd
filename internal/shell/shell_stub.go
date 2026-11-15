//go:build !linux || !cgo

package shell

import "fmt"

func openShellPlugin(path, id string) (shellSurface, error) {
	return nil, fmt.Errorf("shell: dlopen %s: not supported on this platform", path)
}

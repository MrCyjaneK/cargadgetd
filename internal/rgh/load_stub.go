//go:build !linux || !cgo

package rgh

import "fmt"

// Open is unavailable without linux+cgo.
func Open(path string) (*Material, error) {
	path = ResolvePath(path)
	return nil, &LoadError{Path: path, Err: fmt.Errorf("not supported on this platform")}
}

//go:build !linux

package wpp

import "errors"

func Wait(string) error {
	return errors.New("wpp: bluetooth wait is linux only")
}

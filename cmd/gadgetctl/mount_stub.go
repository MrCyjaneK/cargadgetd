//go:build !linux

package main

import "fmt"

func mountConfigFS() error {
	return fmt.Errorf("configfs mount: not linux")
}

func mountFunctionFS(source, target string) error {
	return fmt.Errorf("functionfs mount: not linux")
}

func unmountDetach(target string) error {
	return fmt.Errorf("unmount: not linux")
}

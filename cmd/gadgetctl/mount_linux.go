//go:build linux

package main

import "syscall"

func mountConfigFS() error {
	return syscall.Mount("none", "/sys/kernel/config", "configfs", 0, "")
}

func mountFunctionFS(source, target string) error {
	return syscall.Mount(source, target, "functionfs", 0, "")
}

func unmountDetach(target string) error {
	return syscall.Unmount(target, syscall.MNT_DETACH)
}

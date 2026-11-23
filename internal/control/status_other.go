//go:build !linux

package control

func MDRunning() bool  { return false }
func AOARunning() bool { return false }

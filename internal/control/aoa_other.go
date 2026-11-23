//go:build !linux

package control

func AoaStart() string   { return "Handshake start failed: unsupported" }
func AoaStop() string    { return "Handshake stop failed: unsupported" }
func AoaRestart() string { return "Handshake restart failed: unsupported" }

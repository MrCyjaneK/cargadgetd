//go:build !linux

package ffs

// WithoutAsyncPreempt is a no-op off Linux (no FunctionFS).
func WithoutAsyncPreempt(fn func()) { fn() }

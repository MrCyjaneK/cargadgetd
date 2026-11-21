//go:build !linux

package wpp

import (
	"context"
	"fmt"
	"io"
)

func Dial(ctx context.Context, mac string) (io.ReadWriteCloser, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("wpp: bluetooth dial is linux only")
}

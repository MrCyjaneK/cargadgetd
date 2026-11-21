//go:build !linux

package wifi

import (
	"context"
	"fmt"
)

func Join(ctx context.Context, ssid, pass string) error {
	return Connect(ctx, ssid, pass)
}

func Connect(ctx context.Context, ssid, pass string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("wifi: connman join is linux only")
}

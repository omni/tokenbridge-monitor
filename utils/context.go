package utils

import (
	"context"
	"time"
)

func ContextSleep(ctx context.Context, d time.Duration) *time.Time {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil
	case t := <-timer.C:
		return &t
	}
}

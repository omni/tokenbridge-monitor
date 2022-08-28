package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/omni/tokenbridge-monitor/utils"
)

func TestContextSleep(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dur := 10 * time.Millisecond

	st := time.Now()
	utils.ContextSleep(ctx, dur)
	diff := time.Since(st)

	require.Greater(t, diff, dur)
}

func TestContextSleepCancel(t *testing.T) {
	t.Parallel()

	dur := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), dur)

	st := time.Now()
	utils.ContextSleep(ctx, dur*3)
	diff := time.Since(st)

	require.Greater(t, diff, dur)
	require.Less(t, time.Since(st), dur*2)
	cancel()
}

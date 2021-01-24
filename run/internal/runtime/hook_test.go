package runtime_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe/run/internal/runtime"
)

func TestHooks(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		hookOk := func(context.Context) error {
			return nil
		}
		err := runtime.StartFunc(hookOk).Start(ctx)
		assertEqual(t, "start error", err, nil)
		err = runtime.FlushFunc(hookOk).Flush(ctx)
		assertEqual(t, "flush error", err, nil)
	}

	testNil := func(t *testing.T) {
		ctx := context.Background()
		var start runtime.StartFunc
		err := start.Start(ctx)
		assertEqual(t, "start error", err, nil)
		var flush runtime.FlushFunc
		err = flush.Flush(ctx)
		assertEqual(t, "flush error", err, nil)
	}

	t.Run("hooks ok", testOk)
	t.Run("hooks nil", testNil)
}

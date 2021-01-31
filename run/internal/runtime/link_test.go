package runtime_test

import (
	"pipelined.dev/pipe/run/internal/runtime"
)

var (
	asyncLink runtime.Link = runtime.AsyncLink()
	syncLink  runtime.Link = runtime.SyncLink()
)

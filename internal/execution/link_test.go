package execution_test

import (
	"pipelined.dev/pipe/internal/execution"
)

var (
	asyncLink execution.Link = execution.AsyncLink()
	syncLink  execution.Link = execution.SyncLink()
)

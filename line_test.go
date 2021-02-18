package pipe_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

func TestLine(t *testing.T) {
	l := pipe.Line{
		Source: (&mock.Source{}).Source(),
		Sink:   (&mock.Sink{}).Sink(),
	}

	r, err := l.Runner(bufferSize, nil)
	assertNil(t, "runner error", err)

	err = r.Run(context.Background())
	assertNil(t, "run error", err)
}

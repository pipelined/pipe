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

	err := pipe.Run(context.Background(), bufferSize, l)
	assertNil(t, "run error", err)
}

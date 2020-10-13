package pipe_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

func TestInsertProcessor(t *testing.T) {
	bufferSize := 2
	sink := &mock.Sink{Discard: true}
	p, err := pipe.New(bufferSize, pipe.Routing{
		Source: (&mock.Source{
			Limit:    1024,
			Channels: 2,
		}).Source(),
		Sink: sink.Sink(),
	})
	assertNil(t, "error", err)

	l := p.Lines[0]
	proc := &mock.Processor{}
	a := p.Async(context.Background())

	err = a.AddProcessor(l, 0, proc.Processor())
	assertNil(t, "error", err)
	err = a.Await()
	assertNil(t, "error", err)

	assertEqual(t, "processed", proc.Counter.Messages > 0, true)
	// fmt.Printf("proc: %v\n", proc.Counter)
	// fmt.Printf("sink: %v\n", sink.Counter)
}

func TestAddLine(t *testing.T) {
	source1 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink1 := &mock.Sink{Discard: true}
	route1 := pipe.Routing{
		Source: source1.Source(),
		Sink:   sink1.Sink(),
	}

	source2 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
		Value:    2,
	}
	sink2 := &mock.Sink{Discard: true}
	route2 := pipe.Routing{
		Source: source2.Source(),
		Sink:   sink2.Sink(),
	}

	p, err := pipe.New(
		bufferSize,
		route1,
	)
	assertNil(t, "error", err)

	r := p.Async(context.Background())
	l, err := p.AddRoute(route2)
	assertNil(t, "error", err)
	r.Push(source2.Reset())
	r.Push(r.AddLine(l))

	// start
	err = r.Await()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
}

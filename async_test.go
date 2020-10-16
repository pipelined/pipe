package pipe_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

func TestInsertProcessor(t *testing.T) {
	bufferSize := 2
	testInsert := func(pos int) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			p, err := pipe.New(bufferSize, pipe.Routing{
				Source: (&mock.Source{
					Limit:    500,
					Channels: 2,
				}).Source(),
				Processors: pipe.Processors((&mock.Processor{}).Processor()),
				Sink:       (&mock.Sink{Discard: true}).Sink(),
			})
			assertNil(t, "pipe error", err)

			l := p.Lines[0]
			a := p.Async(context.Background())

			proc := &mock.Processor{}
			err = l.Insert(pos, proc.Processor())
			assertNil(t, "add error", err)

			a.Push(a.StartProcessor(l, pos)...)
			err = a.Await()
			assertNil(t, "await error", err)
			assertEqual(t, "processed", proc.Counter.Messages > 0, true)
		}
	}
	t.Run("before processor", testInsert(0))
	t.Run("before sink", testInsert(1))
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
	l, err := p.Append(route2)
	assertNil(t, "error", err)
	r.Push(source2.Reset())
	r.Push(r.Append(l))

	// start
	err = r.Await()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
}

package pipe_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

// func TestInsertProcessor(t *testing.T) {
// 	bufferSize := 2
// 	testInsert := func(pos int) func(*testing.T) {
// 		return func(t *testing.T) {
// 			t.Helper()
// 			p, err := pipe.New(bufferSize, pipe.Routing{
// 				Source: (&mock.Source{
// 					Limit:    500,
// 					Channels: 2,
// 				}).Source(),
// 				Processors: pipe.Processors((&mock.Processor{}).Processor()),
// 				Sink:       (&mock.Sink{Discard: true}).Sink(),
// 			})
// 			assertNil(t, "pipe error", err)

// 			l := p.Lines[0]
// 			a := p.Async(context.Background())

// 			proc := &mock.Processor{}
// 			err = l.InsertProcessor(pos, proc.Processor())
// 			assertNil(t, "add error", err)

// 			<-a.StartProcessor(l, pos)
// 			err = a.Await()
// 			assertNil(t, "await error", err)
// 			assertEqual(t, "processed", proc.Counter.Messages > 0, true)
// 		}
// 	}
// 	t.Run("before processor", testInsert(0))
// 	t.Run("before sink", testInsert(1))
// }

// func TestInsertMultiple(t *testing.T) {
// 	bufferSize := 2
// 	insert := func(pos int) func(*testing.T) {
// 		return func(t *testing.T) {
// 			t.Helper()
// 			samples := 500
// 			sink := &mock.Sink{Discard: true}
// 			p, err := pipe.New(bufferSize, pipe.Routing{
// 				Source: (&mock.Source{
// 					Limit:    samples,
// 					Channels: 2,
// 				}).Source(),
// 				Processors: pipe.Processors((&mock.Processor{}).Processor()),
// 				Sink:       sink.Sink(),
// 			})
// 			assertNil(t, "pipe error", err)

// 			l := p.Lines[0]
// 			a := p.Async(context.Background())

// 			proc1 := &mock.Processor{}
// 			err = l.InsertProcessor(pos, proc1.Processor())
// 			assertNil(t, "add 1 error", err)
// 			<-a.StartProcessor(l, pos)

// 			proc2 := &mock.Processor{}
// 			err = l.InsertProcessor(pos, proc2.Processor())
// 			assertNil(t, "add 2 error", err)
// 			<-a.StartProcessor(l, pos)

// 			err = a.Await()
// 			assertNil(t, "await error", err)
// 			assertEqual(t, "sink processed", sink.Counter.Samples, samples)
// 			assertEqual(t, "processed 1", proc1.Counter.Messages > 0, true)
// 			assertEqual(t, "processed 2", proc2.Counter.Messages > 0, true)
// 		}
// 	}
// 	t.Run("before processor", insert(0))
// 	t.Run("before sink", insert(1))
// }

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

	// start
	r := p.Run(context.Background())
	l, err := p.AddLine(route2)
	assertNil(t, "error", err)
	<-r.StartLine(l)

	err = r.Wait()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
}

package pipe_test

import (
	"errors"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

func TestLineBindingFail(t *testing.T) {
	var (
		errorBinding = errors.New("binding error")
		bufferSize   = 512
	)
	testBinding := func(l pipe.Routing) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			_, err := pipe.New(bufferSize, l)
			if err == nil {
				return
			}
			if !errors.Is(err, errorBinding) {
				t.Fatal("error")
			}
		}
	}
	t.Run("source", testBinding(
		pipe.Routing{
			Source: (&mock.Source{
				ErrorOnMake: errorBinding,
			}).Source(),
			Processors: pipe.Processors(
				(&mock.Processor{}).Processor(),
			),
			Sink: (&mock.Sink{}).Sink(),
		},
	))
	t.Run("processor", testBinding(
		pipe.Routing{
			Source: (&mock.Source{}).Source(),
			Processors: pipe.Processors(
				(&mock.Processor{
					ErrorOnMake: errorBinding,
				}).Processor(),
			),
			Sink: (&mock.Sink{}).Sink(),
		},
	))
	t.Run("sink", testBinding(
		pipe.Routing{
			Source: (&mock.Source{}).Source(),
			Processors: pipe.Processors(
				(&mock.Processor{}).Processor(),
			),
			Sink: (&mock.Sink{
				ErrorOnMake: errorBinding,
			}).Sink(),
		},
	))
	t.Run("ok", testBinding(
		pipe.Routing{
			Source: (&mock.Source{}).Source(),
			Processors: pipe.Processors(
				(&mock.Processor{}).Processor(),
			),
			Sink: (&mock.Sink{}).Sink(),
		},
	))
}

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

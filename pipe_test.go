package pipe_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutable"
)

const (
	bufferSize  = 512
	pipeTimeout = 1 * time.Second
)

func TestLineBindingFail(t *testing.T) {
	var (
		errorBinding = errors.New("binding error")
		bufferSize   = 512
	)
	testBinding := func(l pipe.Line) func(*testing.T) {
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
		pipe.Line{
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
		pipe.Line{
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
		pipe.Line{
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
		pipe.Line{
			Source: (&mock.Source{}).Source(),
			Processors: pipe.Processors(
				(&mock.Processor{}).Processor(),
			),
			Sink: (&mock.Sink{}).Sink(),
		},
	))
}

func TestSimplePipe(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}

	proc1 := &mock.Processor{}
	sink1 := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Line{
			Source:     source.Source(),
			Processors: pipe.Processors(proc1.Processor()),
			Sink:       sink1.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	err = pipe.Wait(p.Start(context.Background()))
	assertNil(t, "pipe error", err)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestReset(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Line{
			Source: source.Source(),
			Sink:   sink.Sink(),
		},
	)
	assertNil(t, "error", err)

	waitPipe(t, p, pipeTimeout)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)

	waitPipe(t, p, pipeTimeout, source.Reset())
	assertEqual(t, "messages second run", sink.Counter.Messages, 2*862)
	assertEqual(t, "samples second run", sink.Counter.Samples, 2*862*bufferSize)
}

func TestSync(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Line{
			Context: mutable.Mutable(),
			Source:  source.Source(),
			Sink:    sink.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	waitPipe(t, p, pipeTimeout)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestMultiple(t *testing.T) {
	source1 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink1 := &mock.Sink{Discard: true}
	source2 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink2 := &mock.Sink{Discard: true}

	mctx := mutable.Mutable()
	p, err := pipe.New(
		bufferSize,
		pipe.Line{
			Context: mctx,
			Source:  source1.Source(),
			Sink:    sink1.Sink(),
		},
		pipe.Line{
			Context: mctx,
			Source:  source2.Source(),
			Sink:    sink2.Sink(),
		},
	)
	assertNil(t, "error", err)
	// start
	waitPipe(t, p, pipeTimeout)
	assertEqual(t, "messages", source1.Counter.Messages, 862)
	assertEqual(t, "samples", source1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", source2.Counter.Messages, 862)
	assertEqual(t, "samples", source2.Counter.Samples, 862*bufferSize)
}

func TestLines(t *testing.T) {
	type (
		mockLine struct {
			*mock.Source
			*mock.Processor
			*mock.Sink
		}

		assertFunc func(t *testing.T, err error, mocks ...mockLine)
	)
	mockError := fmt.Errorf("mock error")

	assertLine := func(t *testing.T, m mockLine, messages, samples int) {
		assertEqual(t, "src messages", m.Source.Counter.Messages, messages)
		assertEqual(t, "proc messages", m.Processor.Counter.Messages, messages)
		assertEqual(t, "sink messages", m.Sink.Counter.Messages, messages)
		assertEqual(t, "src samples", m.Source.Counter.Samples, samples)
		assertEqual(t, "proc samples", m.Processor.Counter.Samples, samples)
		assertEqual(t, "sink samples", m.Sink.Counter.Samples, samples)
	}

	testLines := func(assertFn assertFunc, mocks ...mockLine) func(*testing.T) {
		return func(t *testing.T) {
			lines := make([]*pipe.LineRunner, 0, len(mocks))
			for i := range mocks {
				r, err := pipe.Line{
					Source:     mocks[i].Source.Source(),
					Processors: pipe.Processors(mocks[i].Processor.Processor()),
					Sink:       mocks[i].Sink.Sink(),
				}.Runner(bufferSize, nil)
				assertNil(t, "runner error", err)
				lines = append(lines, r)
			}
			runner := pipe.MultiLineRunner{
				Lines: lines,
			}
			err := runner.Run(context.Background())
			assertFn(t, err, mocks...)
		}
	}

	t.Run("two lines processor start error source flush error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			m := mocks[0]
			assertEqual(t, "line 1 src started", m.Source.Started, true)
			assertEqual(t, "line 1 proc started", m.Processor.Started, true)
			assertEqual(t, "line 1 sink started", m.Sink.Started, true)
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			m = mocks[1]
			assertEqual(t, "line 2 src started", m.Source.Started, true)
			assertEqual(t, "line 2 proc started", m.Processor.Started, true)
			assertEqual(t, "line 2 sink started", m.Sink.Started, false)
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
				Flusher: mock.Flusher{
					ErrorOnFlush: mockError,
				},
			},
			Processor: &mock.Processor{},
			Sink:      &mock.Sink{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("two lines processor start error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			m := mocks[0]
			assertEqual(t, "src started", m.Source.Started, true)
			assertEqual(t, "proc started", m.Processor.Started, true)
			assertEqual(t, "sink started", m.Sink.Started, true)
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			m = mocks[1]
			assertEqual(t, "line 2 src started", m.Source.Started, true)
			assertEqual(t, "line 2 proc started", m.Processor.Started, true)
			assertEqual(t, "line 2 sink started", m.Sink.Started, false)
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{},
			Sink:      &mock.Sink{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("single line processor start error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			m := mocks[0]
			assertEqual(t, "line 1 src started", m.Source.Started, true)
			assertEqual(t, "line 1 proc started", m.Processor.Started, true)
			assertEqual(t, "line 1 sink started", m.Sink.Started, false)
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("single line ok", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			m := mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 3, 1040)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
	t.Run("two lines ok", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			var m mockLine
			m = mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 3, 1040)
			m = mocks[1]
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 4, 1640)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1640,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
	t.Run("three lines ok", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			var m mockLine
			m = mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 6, 3048)
			m = mocks[1]
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 4, 1640)
			m = mocks[2]
			assertEqual(t, "line 3 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 3 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 3 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 8, 4096)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    3048,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1640,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    4096,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
	t.Run("single processor error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "error", errors.Is(err, mockError), true)
			m := mocks[0]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{
				ErrorOnCall: mockError,
			},
		},
	))

}

// func TestAddLine(t *testing.T) {
// 	addLine := func(async bool) func(*testing.T) {
// 		return func(t *testing.T) {
// 			sink1 := &mock.Sink{Discard: true}
// 			line1 := pipe.Line{
// 				Source: (&mock.Source{
// 					Limit:    862 * bufferSize,
// 					Channels: 2,
// 				}).Source(),
// 				Sink: sink1.Sink(),
// 			}

// 			var mut mutable.Context
// 			if !async {
// 				mut = mutable.Mutable()
// 			}
// 			sink2 := &mock.Sink{Discard: true}
// 			line2 := pipe.Line{
// 				Context: mut,
// 				Source: (&mock.Source{
// 					Limit:    862 * bufferSize,
// 					Channels: 2,
// 					Value:    2,
// 				}).Source(),
// 				Sink: sink2.Sink(),
// 			}

// 			p, err := pipe.New(
// 				bufferSize,
// 				line1,
// 			)
// 			assertNil(t, "error", err)

// 			// start
// 			errc := p.Start(context.Background())
// 			p.Push(p.AddLine(line2))
// 			assertNil(t, "error", err)

// 			err = pipe.Wait(errc)
// 			assertEqual(t, "messages", sink1.Counter.Messages, 862)
// 			assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
// 			assertEqual(t, "messages", sink2.Counter.Messages, 862)
// 			assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
// 		}
// 	}
// 	t.Run("async", addLine(true))
// 	t.Run("sync", addLine(false))
// }

// func TestAddLineMultiLine(t *testing.T) {
// 	sink1 := &mock.Sink{Discard: true}
// 	line1 := pipe.Line{
// 		Source: (&mock.Source{
// 			Limit:    862 * bufferSize,
// 			Channels: 2,
// 		}).Source(),
// 		Sink: sink1.Sink(),
// 	}

// 	mut := mutable.Mutable()
// 	sink2 := &mock.Sink{Discard: true}
// 	line2 := pipe.Line{
// 		Context: mut,
// 		Source: (&mock.Source{
// 			Limit:    862 * bufferSize,
// 			Channels: 2,
// 			Value:    2,
// 		}).Source(),
// 		Sink: sink2.Sink(),
// 	}
// 	sink3 := &mock.Sink{Discard: true}
// 	line3 := pipe.Line{
// 		Context: mut,
// 		Source: (&mock.Source{
// 			Limit:    862 * bufferSize,
// 			Channels: 2,
// 			Value:    2,
// 		}).Source(),
// 		Sink: sink3.Sink(),
// 	}
// 	sink4 := &mock.Sink{Discard: true}
// 	line4 := pipe.Line{
// 		Context: mut,
// 		Source: (&mock.Source{
// 			Limit:    862 * bufferSize,
// 			Channels: 2,
// 			Value:    2,
// 		}).Source(),
// 		Sink: sink4.Sink(),
// 	}

// 	p, err := pipe.New(
// 		bufferSize,
// 		line1,
// 		line2,
// 	)
// 	assertNil(t, "error", err)

// 	// start
// 	errc := p.Start(context.Background())
// 	p.Push(p.AddLine(line3), p.AddLine(line4))
// 	assertNil(t, "error", err)

// 	err = pipe.Wait(errc)
// 	assertEqual(t, "messages", sink1.Counter.Messages, 862)
// 	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
// 	assertEqual(t, "messages", sink2.Counter.Messages, 862)
// 	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
// 	assertEqual(t, "messages", sink3.Counter.Messages, 862)
// 	assertEqual(t, "samples", sink3.Counter.Samples, 862*bufferSize)
// 	assertEqual(t, "messages", sink4.Counter.Messages, 862)
// 	assertEqual(t, "samples", sink4.Counter.Samples, 862*bufferSize)
// }

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

func waitPipe(t *testing.T, p *pipe.Pipe, timeout time.Duration, inits ...mutable.Mutation) {
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	errc := p.Start(context.Background(), inits...)

	select {
	case err := <-errc:
		assertNil(t, "pipe error", err)
	case <-ctx.Done():
		t.Fatalf("pipe timeout reached")
	}
}

func assertNil(t *testing.T, name string, result interface{}) {
	t.Helper()
	assertEqual(t, name, result, nil)
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

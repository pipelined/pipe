package run_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run"
)

const bufferSize = 512

func TestSimplePipe(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}

	proc1 := &mock.Processor{}
	sink1 := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Routing{
			Source:     source.Source(),
			Processors: pipe.Processors(proc1.Processor()),
			Sink:       sink1.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	r := run.New(context.Background(), p)
	err = r.Wait()
	assertNil(t, "error", err)

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
		pipe.Routing{
			Source: source.Source(),
			Sink:   sink.Sink(),
		},
	)
	assertNil(t, "error", err)
	r := run.New(context.Background(), p)
	// start
	err = r.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)

	r = run.New(context.Background(), p, source.Reset())
	_ = r.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", sink.Counter.Messages, 2*862)
	assertEqual(t, "samples", sink.Counter.Samples, 2*862*bufferSize)
}

func TestSync(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Routing{
			Context: mutable.Mutable(),
			Source:  source.Source(),
			Sink:    sink.Sink(),
		},
	)
	assertNil(t, "error", err)
	r := run.New(context.Background(), p)
	// start
	err = r.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestSyncMultiple(t *testing.T) {
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
		pipe.Routing{
			Context: mctx,
			Source:  source1.Source(),
			Sink:    sink1.Sink(),
		},
		pipe.Routing{
			Context: mctx,
			Source:  source2.Source(),
			Sink:    sink2.Sink(),
		},
	)
	assertNil(t, "error", err)
	r := run.New(context.Background(), p)
	// start
	err = r.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", source1.Counter.Messages, 862)
	assertEqual(t, "samples", source1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", source2.Counter.Messages, 862)
	assertEqual(t, "samples", source2.Counter.Samples, 862*bufferSize)
}

// This benchmark runs next line:
// 1 Source, 2 Processors, 1 Sink, 862 buffers of 512 samples with 2 channels.
func BenchmarkSingleLine(b *testing.B) {
	source := &mock.Source{
		Mutator: mock.Mutator{
			Mutability: mutable.Mutable(),
		},
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}
	p, _ := pipe.New(bufferSize, pipe.Routing{
		Source: source.Source(),
		Processors: pipe.Processors(
			(&mock.Processor{}).Processor(),
			(&mock.Processor{}).Processor(),
		),
		Sink: sink.Sink(),
	})
	for i := 0; i < b.N; i++ {
		_ = run.New(context.Background(), p, source.Reset()).Wait()
	}
	b.Logf("recieved messages: %d samples: %d", sink.Messages, sink.Samples)
}

func TestLineBindingFail(t *testing.T) {
	var (
		errorBinding = errors.New("binding error")
		bufferSize   = 512
	)
	testBinding := func(l pipe.Routing) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			_, err := pipe.New(bufferSize, l)
			assertEqual(t, "error", errors.Is(err, errorBinding), true)
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

	// start
	r := run.New(context.Background(), p)
	l, err := p.AddLine(route2)
	assertNil(t, "error", err)
	<-r.StartLine(l)

	err = r.Wait()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
}

func TestAddLineSyncSameContext(t *testing.T) {
	mctx := mutable.Mutable()
	source1 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink1 := &mock.Sink{Discard: true}
	p, err := pipe.New(
		bufferSize,
		pipe.Routing{
			Context: mctx,
			Source:  source1.Source(),
			Sink:    sink1.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	r := run.New(context.Background(), p)

	source2 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
		Value:    2,
	}
	sink2 := &mock.Sink{Discard: true}
	l, err := p.AddLine(pipe.Routing{
		Context: mctx,
		Source:  source2.Source(),
		Sink:    sink2.Sink(),
	})
	assertNil(t, "error", err)
	<-r.StartLine(l)

	err = r.Wait()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
	assertEqual(t, "sink 2 started", sink2.Started, true)
	assertEqual(t, "sink 2 flushed", sink2.Flushed, true)
}

func TestAddLineSyncNewContext(t *testing.T) {
	source1 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink1 := &mock.Sink{Discard: true}

	p, err := pipe.New(
		bufferSize,
		pipe.Routing{
			Context: mutable.Mutable(),
			Source:  source1.Source(),
			Sink:    sink1.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	r := run.New(context.Background(), p)

	// add second route
	source2 := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
		Value:    2,
	}
	sink2 := &mock.Sink{Discard: true}
	l, err := p.AddLine(pipe.Routing{
		Context: mutable.Mutable(),
		Source:  source2.Source(),
		Sink:    sink2.Sink(),
	})
	assertNil(t, "error", err)
	<-r.StartLine(l)

	err = r.Wait()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
	assertEqual(t, "sink 2 started", sink2.Started, true)
	assertEqual(t, "sink 2 flushed", sink2.Flushed, true)
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

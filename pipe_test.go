package pipe_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/repeat"
)

const (
	bufferSize = 512
)

func TestPipe(t *testing.T) {
	t.Skip()
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	repeater := &repeat.Repeater{}
	sink1 := &mock.Sink{Discard: true}
	sink2 := &mock.Sink{Discard: true}

	in, err := pipe.Route{
		Source:     source.Source(),
		Processors: pipe.Processors(proc1.Processor(), proc2.Processor()),
		Sink:       repeater.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)
	out1, err := pipe.Route{
		Source: repeater.Source(),
		Sink:   sink1.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)
	out2, err := pipe.Route{
		Source: repeater.Source(),
		Sink:   sink2.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)

	p := pipe.New(context.Background(), pipe.WithLines(in, out1, out2))
	// start
	err = p.Wait()
	assertNil(t, "error", err)

	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestSimplePipe(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}

	proc1 := &mock.Processor{}
	sink1 := &mock.Sink{Discard: true}

	in, err := pipe.Route{
		Source:     source.Source(),
		Processors: pipe.Processors(proc1.Processor()),
		Sink:       sink1.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)

	p := pipe.New(context.Background(), pipe.WithLines(in))
	// start
	err = p.Wait()
	assertNil(t, "error", err)

	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestReset(t *testing.T) {
	source := &mock.Source{
		Mutator: mock.Mutator{
			Mutability: mutability.Mutable(),
		},
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}

	route, err := pipe.Route{
		Source: source.Source(),
		Sink:   sink.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)
	p := pipe.New(
		context.Background(),
		pipe.WithLines(route),
	)
	// start
	err = p.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)

	p = pipe.New(
		context.Background(),
		pipe.WithLines(route),
		pipe.WithMutations(source.Reset()),
	)
	_ = p.Wait()
	assertNil(t, "error", err)
	assertEqual(t, "messages", sink.Counter.Messages, 2*862)
	assertEqual(t, "samples", sink.Counter.Samples, 2*862*bufferSize)
}

func TestAddLine(t *testing.T) {
	sink1 := &mock.Sink{Discard: true}
	route1, err := pipe.Route{
		Source: (&mock.Source{
			Limit:    862 * bufferSize,
			Channels: 2,
		}).Source(),
		Sink: sink1.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)

	sink2 := &mock.Sink{Discard: true}
	route2, err := pipe.Route{
		Source: (&mock.Source{
			Limit:    862 * bufferSize,
			Channels: 2,
		}).Source(),
		Sink: sink2.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)

	p := pipe.New(
		context.Background(),
		pipe.WithLines(route1),
	)
	p.Push(p.AddLine(route2))

	// start
	err = p.Wait()
	assertEqual(t, "messages", sink1.Counter.Messages, 862)
	assertEqual(t, "samples", sink1.Counter.Samples, 862*bufferSize)
	assertEqual(t, "messages", sink2.Counter.Messages, 862)
	assertEqual(t, "samples", sink2.Counter.Samples, 862*bufferSize)
}

// This benchmark runs next line:
// 1 Source, 2 Processors, 1 Sink, 862 buffers of 512 samples with 2 channels.
func BenchmarkSingleLine(b *testing.B) {
	source := &mock.Source{
		Mutator: mock.Mutator{
			Mutability: mutability.Mutable(),
		},
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	sink := &mock.Sink{Discard: true}
	line, _ := pipe.Route{
		Source: source.Source(),
		Processors: pipe.Processors(
			(&mock.Processor{}).Processor(),
			(&mock.Processor{}).Processor(),
		),
		Sink: sink.Sink(),
	}.Line(bufferSize)
	for i := 0; i < b.N; i++ {
		p := pipe.New(
			context.Background(),
			pipe.WithLines(line),
			pipe.WithMutations(source.Reset()),
		)
		_ = p.Wait()
	}
	b.Logf("recieved messages: %d samples: %d", sink.Messages, sink.Samples)
}

func TestLineBindingFail(t *testing.T) {
	var (
		errorBinding = errors.New("binding error")
		bufferSize   = 512
	)
	testBinding := func(r pipe.Route) func(*testing.T) {
		return func(t *testing.T) {
			_, err := r.Line(bufferSize)
			assertEqual(t, "error", errors.Is(err, errorBinding), true)
		}
	}
	t.Run("source", testBinding(
		pipe.Route{
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
		pipe.Route{
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
		pipe.Route{
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

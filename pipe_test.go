package pipe_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/repeat"
)

const (
	bufferSize = 512
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPipe(t *testing.T) {
	t.Skip()
	pump := &mock.Pump{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	repeater := &repeat.Repeater{}
	sink1 := &mock.Sink{Discard: true}
	sink2 := &mock.Sink{Discard: true}

	in, err := pipe.Route{
		Pump:       pump.Pump(),
		Processors: pipe.Processors(proc1.Processor(), proc2.Processor()),
		Sink:       repeater.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)
	out1, err := pipe.Route{
		Pump: repeater.Pump(),
		Sink: sink1.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)
	out2, err := pipe.Route{
		Pump: repeater.Pump(),
		Sink: sink2.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)

	p := pipe.New(context.Background(), pipe.WithLines(in, out1, out2))
	// start
	err = p.Wait()
	assert.Nil(t, err)

	assert.Equal(t, 862, pump.Counter.Messages)
	assert.Equal(t, 862*bufferSize, pump.Counter.Samples)
}

func TestSimplePipe(t *testing.T) {
	pump := &mock.Pump{
		Limit:    862 * bufferSize,
		Channels: 2,
	}

	proc1 := &mock.Processor{}
	sink1 := &mock.Sink{Discard: true}

	in, err := pipe.Route{
		Pump:       pump.Pump(),
		Processors: pipe.Processors(proc1.Processor()),
		Sink:       sink1.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)

	p := pipe.New(context.Background(), pipe.WithLines(in))
	// start
	err = p.Wait()
	assert.Nil(t, err)

	assert.Equal(t, 862, pump.Counter.Messages)
	assert.Equal(t, 862*bufferSize, pump.Counter.Samples)
}

func TestReset(t *testing.T) {
	pump := &mock.Pump{
		Mutability: mutability.Mutable(),
		Limit:      862 * bufferSize,
		Channels:   2,
	}
	sink := &mock.Sink{Discard: true}

	route, err := pipe.Route{
		Pump: pump.Pump(),
		Sink: sink.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)
	p := pipe.New(
		context.Background(),
		pipe.WithLines(route),
	)
	// start
	err = p.Wait()
	assert.Nil(t, err)
	assert.Equal(t, 862, pump.Counter.Messages)
	assert.Equal(t, 862*bufferSize, pump.Counter.Samples)

	p = pipe.New(
		context.Background(),
		pipe.WithLines(route),
		pipe.WithMutations(pump.Reset()),
	)
	_ = p.Wait()
	assert.Nil(t, err)
	assert.Equal(t, 2*862, sink.Counter.Messages)
	assert.Equal(t, 2*862*bufferSize, sink.Counter.Samples)
}

func TestAddLine(t *testing.T) {
	sink1 := &mock.Sink{Discard: true}
	route1, err := pipe.Route{
		Pump: (&mock.Pump{
			Limit:    862 * bufferSize,
			Channels: 2,
		}).Pump(),
		Sink: sink1.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)

	sink2 := &mock.Sink{Discard: true}
	route2, err := pipe.Route{
		Pump: (&mock.Pump{
			Limit:    862 * bufferSize,
			Channels: 2,
		}).Pump(),
		Sink: sink2.Sink(),
	}.Line(bufferSize)
	assert.Nil(t, err)

	p := pipe.New(
		context.Background(),
		pipe.WithLines(route1),
	)
	p.Push(p.AddLine(route2))

	// start
	err = p.Wait()
	assert.Equal(t, 862, sink1.Counter.Messages)
	assert.Equal(t, 862*bufferSize, sink1.Counter.Samples)
	assert.Equal(t, 862, sink2.Counter.Messages)
	assert.Equal(t, 862*bufferSize, sink2.Counter.Samples)
}

// This benchmark runs next line:
// 1 Pump, 2 Processors, 1 Sink, 862 buffers of 512 samples with 2 channels.
func BenchmarkSingleLine(b *testing.B) {
	pump := &mock.Pump{
		Mutability: mutability.Mutable(),
		Limit:      862 * bufferSize,
		Channels:   2,
	}
	sink := &mock.Sink{Discard: true}
	route, _ := pipe.Route{
		Pump: pump.Pump(),
		Processors: pipe.Processors(
			(&mock.Processor{}).Processor(),
			(&mock.Processor{}).Processor(),
		),
		Sink: sink.Sink(),
	}.Line(bufferSize)
	for i := 0; i < b.N; i++ {
		p := pipe.New(
			context.Background(),
			pipe.WithLines(route),
			pipe.WithMutations(pump.Reset()),
		)
		_ = p.Wait()
	}
	b.Logf("recieved messages: %d samples: %d", sink.Messages, sink.Samples)
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

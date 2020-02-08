package pipe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/split"
)

const (
	bufferSize = 512
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPipe(t *testing.T) {
	pumpOptions := &mock.PumpOptions{
		Limit:       862 * bufferSize,
		NumChannels: 2,
	}
	pump := mock.Pump(pumpOptions)
	proc1 := mock.Processor(&mock.ProcessorOptions{})
	proc2 := mock.Processor(&mock.ProcessorOptions{})
	splitter := &split.Splitter{}
	sink1 := mock.Sink(&mock.SinkOptions{Discard: true})
	sink2 := mock.Sink(&mock.SinkOptions{Discard: true})

	in, err := pipe.Line{
		Pump:       pump,
		Processors: pipe.Processors(proc1, proc2),
		Sink:       splitter.Sink(),
	}.Route(bufferSize)
	assert.Nil(t, err)
	out1, err := pipe.Line{
		Pump: splitter.Pump(),
		Sink: sink1,
	}.Route(bufferSize)
	assert.Nil(t, err)
	out2, err := pipe.Line{
		Pump: splitter.Pump(),
		Sink: sink2,
	}.Route(bufferSize)
	assert.Nil(t, err)

	p := pipe.New(in, out1, out2)
	// start
	err = pipe.Wait(p.Run(context.Background()))
	assert.Nil(t, err)

	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	assert.Equal(t, 862, pumpOptions.Counter.Messages)
	assert.Equal(t, 862*bufferSize, pumpOptions.Counter.Samples)
}

func TestSimplePipe(t *testing.T) {
	pumpOptions := &mock.PumpOptions{
		Limit:       862 * bufferSize,
		NumChannels: 2,
	}
	pump := mock.Pump(pumpOptions)
	proc1 := mock.Processor(&mock.ProcessorOptions{})
	sink1 := mock.Sink(&mock.SinkOptions{Discard: true})

	in, err := pipe.Line{
		Pump:       pump,
		Processors: pipe.Processors(proc1),
		Sink:       sink1,
	}.Route(bufferSize)
	assert.Nil(t, err)

	p := pipe.New(in)
	// start
	err = pipe.Wait(p.Run(context.Background()))
	assert.Nil(t, err)

	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	assert.Equal(t, 862, pumpOptions.Counter.Messages)
	assert.Equal(t, 862*bufferSize, pumpOptions.Counter.Samples)
}

// This benchmark runs next line:
// 1 Pump, 2 Processors, 2 Sinks, 1000 buffers of 512 samples with 2 channels.
func BenchmarkSingleLine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		route, _ := pipe.Line{
			Pump: mock.Pump(
				&mock.PumpOptions{
					Limit:       862 * bufferSize,
					NumChannels: 2,
				},
			),
			Processors: pipe.Processors(
				mock.Processor(&mock.ProcessorOptions{}),
				mock.Processor(&mock.ProcessorOptions{}),
			),
			Sink: mock.Sink(&mock.SinkOptions{Discard: true}),
		}.Route(bufferSize)
		l := pipe.New(route)
		pipe.Wait(l.Run(context.Background()))
		pipe.Wait(l.Close())
	}
}

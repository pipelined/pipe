package pipe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
)

const (
	bufferSize = 512
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPipe(t *testing.T) {
	pump := mock.Pump(&mock.PumpOptions{
		Limit:       862 * bufferSize,
		NumChannels: 2,
	})
	proc1 := mock.Processor(&mock.ProcessorOptions{})
	proc2 := mock.Processor(&mock.ProcessorOptions{})
	sink1 := mock.Sink(&mock.SinkOptions{Discard: true})
	sink2 := mock.Sink(&mock.SinkOptions{Discard: true})

	l := pipe.New(
		&pipe.Line{
			Pump:       pump,
			Processors: pipe.Processors(proc1, proc2),
			Sinks:      pipe.Sinks(sink1, sink2),
		},
	)

	// start
	runc := l.Run(context.Background(), bufferSize)
	assert.NotNil(t, runc)

	// pause
	err := pipe.Wait(l.Pause())
	assert.Nil(t, err)
	// runc must be cancelled by now
	err = pipe.Wait(runc)
	assert.Nil(t, err)

	// resume
	err = pipe.Wait(l.Resume())
	assert.Nil(t, err)

	pipe.Wait(l.Close())
}

// This benchmark runs next line:
// 1 Pump, 2 Processors, 2 Sinks, 1000 buffers of 512 samples with 2 channels.
func BenchmarkSingleLine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		l := pipe.New(
			&pipe.Line{
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
				Sinks: pipe.Sinks(
					mock.Sink(&mock.SinkOptions{Discard: true}),
					mock.Sink(&mock.SinkOptions{Discard: true}),
				),
			},
		)
		pipe.Wait(l.Run(context.Background(), bufferSize))
		pipe.Wait(l.Close())
	}
}

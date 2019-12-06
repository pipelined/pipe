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
	pump := &mock.Pump{
		Limit:       100 * bufferSize,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}

	l, err := pipe.New(
		&pipe.Line{
			Pump:       pump,
			Processors: pipe.Processors(proc1, proc2),
			Sinks:      pipe.Sinks(sink1, sink2),
		},
	)
	assert.Nil(t, err)

	// start
	runc := l.Run(context.Background(), bufferSize)
	assert.NotNil(t, runc)
	assert.Nil(t, err)

	// test params push
	pumpID, ok := l.ComponentID(pump)
	assert.True(t, ok)
	assert.NotEmpty(t, pumpID)
	// push new limit for pump
	newLimit := 200
	paramFn := pump.LimitParam(newLimit)
	l.Push(pumpID, paramFn)

	// pause
	err = pipe.Wait(l.Pause())
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
		pump := &mock.Pump{
			Limit:       862 * bufferSize,
			NumChannels: 2,
		}
		proc1 := &mock.Processor{}
		proc2 := &mock.Processor{}
		sink1 := &mock.Sink{
			Discard: true,
		}
		sink2 := &mock.Sink{
			Discard: true,
		}

		l, _ := pipe.New(
			&pipe.Line{
				Pump:       pump,
				Processors: pipe.Processors(proc1, proc2),
				Sinks:      pipe.Sinks(sink1, sink2),
			},
		)
		pipe.Wait(l.Run(context.Background(), bufferSize))
		pipe.Wait(l.Close())
	}
}

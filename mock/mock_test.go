package mock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
)

var (
	tests = []struct {
		BufferSize  int
		NumChannels int
		mock.Limit
		value    float64
		messages int
		samples  int
	}{
		{
			NumChannels: 1,
			Limit:       10,
			value:       0.5,
			messages:    10,
			samples:     100,
		},
		{
			NumChannels: 2,
			Limit:       100,
			value:       0.7,
			messages:    100,
			samples:     1000,
		},
	}
	bufferSize = 10
	sampleRate = 10
)

func TestPipe(t *testing.T) {
	pump := &mock.Pump{
		// UID:        phono.NewUID(),
		Limit:      1,
		BufferSize: bufferSize,
	}
	processor := &mock.Processor{}
	sink := &mock.Sink{}
	p, err := pipe.New(
		sampleRate,
		pipe.WithName("Mock"),
		pipe.WithPump(pump),
		pipe.WithProcessors(processor),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	for _, test := range tests {
		p.Push(pump,
			pump.LimitParam(test.Limit),
			pump.NumChannelsParam(test.NumChannels),
			pump.ValueParam(test.value),
		)
		err := pipe.Wait(p.Run())
		assert.Nil(t, err)

		messageCount, samplesCount := pump.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		messageCount, samplesCount = processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		messageCount, samplesCount = sink.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		assert.Equal(t, test.NumChannels, sink.Buffer.NumChannels())
		assert.Equal(t, test.samples, sink.Buffer.Size())
	}
}

func TestComponentsReuse(t *testing.T) {
	pump := &mock.Pump{
		Limit:       5,
		Interval:    10 * time.Microsecond,
		BufferSize:  10,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}

	p, err := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	assert.Nil(t, err)

	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	p, err = pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
}

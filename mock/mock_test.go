package mock_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var (
	tests = []struct {
		phono.BufferSize
		phono.NumChannels
		mock.Limit
		value    float64
		messages uint64
		samples  uint64
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
	bufferSize = phono.BufferSize(10)
)

func TestPipe(t *testing.T) {
	pump := &mock.Pump{
		Limit:      1,
		BufferSize: bufferSize,
	}
	processor := &mock.Processor{}
	sink := &mock.Sink{}
	p := pipe.New(
		pipe.WithPump(pump),
		pipe.WithProcessors(processor),
		pipe.WithSinks(sink),
	)

	for _, test := range tests {
		params := phono.NewParams(
			pump.LimitParam(test.Limit),
			pump.NumChannelsParam(test.NumChannels),
			pump.ValueParam(test.value),
		)
		p.Push(params)
		err := pipe.Do(p.Run)
		assert.Nil(t, err)
		err = p.Wait(pipe.Ready)
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
		assert.Equal(t, test.samples, uint64(sink.Buffer.Size()))
	}
}
package asset_test

import (
	"context"
	"testing"

	"github.com/dudk/phono"
	"github.com/dudk/phono/asset"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
	"github.com/stretchr/testify/assert"
)

var (
	tests = []struct {
		phono.BufferSize
		phono.NumChannels
		mock.Limit
		value    float64
		messages int64
		samples  int64
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
		UID:        phono.NewUID(),
		Limit:      1,
		BufferSize: bufferSize,
	}
	processor := &mock.Processor{UID: phono.NewUID()}
	sampleRate := phono.SampleRate(44100)

	for _, test := range tests {
		sink := asset.New()
		p := pipe.New(
			sampleRate,
			pipe.WithName("Mock"),
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		p.Push(
			pump.LimitParam(test.Limit),
			pump.NumChannelsParam(test.NumChannels),
			pump.ValueParam(test.value),
		)
		err := pipe.Wait(p.Run(context.Background()))
		assert.Nil(t, err)

		messageCount, samplesCount := pump.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		messageCount, samplesCount = processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		assert.Equal(t, test.NumChannels, sink.Buffer.NumChannels())
		assert.Equal(t, phono.BufferSize(test.samples), sink.Buffer.Size())

		for i := range sink.Buffer {
			for j := range sink.Buffer[i] {
				assert.Equal(t, test.value, sink.Buffer[i][j])
			}
		}
		err = pipe.Wait(p.Run(context.Background()))
		assert.NotNil(t, err)
		assert.Equal(t, phono.ErrSingleUseReused, err)
	}
}

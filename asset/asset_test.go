package asset_test

import (
	"testing"

	"github.com/pipelined/phono/asset"
	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
	"github.com/stretchr/testify/assert"
)

var (
	tests = []struct {
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
)

func TestPipe(t *testing.T) {
	for _, test := range tests {
		pump := &mock.Pump{
			Limit: 1,
		}
		processor := &mock.Processor{}
		sink := &asset.Asset{}
		p, err := pipe.New(
			bufferSize,
			pipe.WithName("Mock"),
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		assert.Nil(t, err)
		p.Push(
			pump,
			pump.LimitParam(test.Limit),
			pump.NumChannelsParam(test.NumChannels),
			pump.ValueParam(test.value),
		)
		err = pipe.Wait(p.Run())
		assert.Nil(t, err)

		messageCount, samplesCount := pump.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		messageCount, samplesCount = processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		asset := sink.Asset()
		assert.Equal(t, test.NumChannels, asset.NumChannels())
		assert.Equal(t, test.samples, asset.Size())

		for i := range asset {
			for j := range asset[i] {
				assert.Equal(t, test.value, asset[i][j])
			}
		}
		err = pipe.Wait(p.Run())
		assert.Nil(t, err)
	}
}

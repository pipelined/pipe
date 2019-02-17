package audio_test

import (
	"testing"

	"github.com/pipelined/mock"
	"github.com/pipelined/phono/audio"
	"github.com/pipelined/phono/pipe"
	"github.com/stretchr/testify/assert"
)

var (
	tests = []struct {
		NumChannels int
		value       float64
		messages    int
		samples     int
	}{
		{
			NumChannels: 1,
			value:       0.5,
			messages:    10,
			samples:     100,
		},
		{
			NumChannels: 2,
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
			Limit:       test.messages,
			NumChannels: test.NumChannels,
			Value:       test.value,
		}
		processor := &mock.Processor{}
		asset := &audio.Asset{}
		p, err := pipe.New(
			bufferSize,
			pipe.WithName("Mock"),
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(asset),
		)
		assert.Nil(t, err)
		err = pipe.Wait(p.Run())
		assert.Nil(t, err)

		messageCount, samplesCount := pump.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		messageCount, samplesCount = processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, samplesCount)

		buf := asset.Data()
		assert.Equal(t, test.NumChannels, buf.NumChannels())
		assert.Equal(t, test.samples, buf.Size())

		for i := range buf {
			for j := range buf[i] {
				assert.Equal(t, test.value, buf[i][j])
			}
		}
		err = pipe.Wait(p.Run())
		assert.Nil(t, err)
	}
}

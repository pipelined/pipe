package mock_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var tests = []struct {
	phono.BufferSize
	mock.Limit
	messages uint64
	samples  uint64
}{
	{
		BufferSize: 10,
		Limit:      10,
		messages:   10,
		samples:    100,
	},
	{
		BufferSize: phono.BufferSize(10),
		Limit:      100,
		messages:   100,
		samples:    1000,
	},
}

func TestPipe(t *testing.T) {
	pump := &mock.Pump{Limit: 1}
	processor := &mock.Processor{}
	sink := &mock.Sink{}
	p := pipe.New(
		pipe.WithPump(pump),
		pipe.WithProcessors(processor),
		pipe.WithSinks(sink),
	)

	for _, test := range tests {
		pump.BufferSize = test.BufferSize
		params := phono.NewParams(pump.LimitParam(test.Limit))
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
	}
}

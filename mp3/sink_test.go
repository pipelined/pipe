package mp3_test

import (
	"testing"

	"github.com/pipelined/phono/mock"

	"github.com/pipelined/phono/mp3"
	"github.com/pipelined/phono/pipe"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
)

func TestSink(t *testing.T) {
	bufferSize := 512
	pump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	assert.Nil(t, err)
	sampleRate := pump.SampleRate()
	sink, err := mp3.NewSink(test.Out.Mp3, pump.SampleRate(), pump.NumChannels(), 192, 2)
	mockSink := &mock.Sink{}
	assert.Nil(t, err)
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(pump),
		pipe.WithSinks(sink, mockSink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
	_, samples := mockSink.Count()
	assert.Equal(t, test.Data.Wav1Samples, samples)
}

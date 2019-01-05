package mp3_test

import (
	"testing"

	"github.com/dudk/phono/mp3"
	"github.com/dudk/phono/pipe"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/test"
	"github.com/dudk/phono/wav"
)

func TestSink(t *testing.T) {
	bufferSize := phono.BufferSize(512)
	pump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	assert.Nil(t, err)
	sampleRate := pump.WavSampleRate()
	sink, err := mp3.NewSink(test.Out.Mp3, pump.WavSampleRate(), pump.WavNumChannels(), 192, 2)
	assert.Nil(t, err)
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
}

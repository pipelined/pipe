// +build portaudio

package portaudio_test

import (
	"testing"

	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/test"

	"github.com/dudk/phono"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = phono.BufferSize(512)
)

func TestSink(t *testing.T) {
	pump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	assert.Nil(t, err)
	sampleRate := pump.WavSampleRate()
	sink := portaudio.NewSink(bufferSize, pump.WavSampleRate(), pump.WavNumChannels())
	assert.Nil(t, err)

	playback, err := pipe.New(
		sampleRate,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	err = pipe.Wait(playback.Run())
	assert.Nil(t, err)
}

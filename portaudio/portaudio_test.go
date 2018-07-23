package portaudio_test

import (
	"testing"

	"github.com/dudk/phono/pipe"

	"github.com/dudk/phono"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = phono.BufferSize(512)
	inFile     = "../_testdata/in1.wav"
)

func TestSink(t *testing.T) {
	pump, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	sink := portaudio.NewSink(bufferSize, pump.WavSampleRate(), pump.WavNumChannels())
	assert.Nil(t, err)

	playback := pipe.New(
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)

	err = playback.Do(playback.Run)
	assert.Nil(t, err)
}

// +build portaudio

package portaudio_test

import (
	"testing"

	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"

	"github.com/pipelined/phono"
	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/wav"
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

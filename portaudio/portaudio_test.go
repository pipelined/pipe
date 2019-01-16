// +build portaudio

package portaudio_test

import (
	"testing"

	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"

	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = 512
)

func TestSink(t *testing.T) {
	pump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	assert.Nil(t, err)
	sink := portaudio.NewSink(bufferSize, pump.SampleRate(), pump.NumChannels())
	assert.Nil(t, err)

	playback, err := pipe.New(
		pump.SampleRate(),
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	err = pipe.Wait(playback.Run())
	assert.Nil(t, err)
}

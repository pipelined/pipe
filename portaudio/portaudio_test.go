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
	pump := wav.NewPump(test.Data.Wav1, bufferSize)
	sink := portaudio.NewSink(bufferSize)

	playback, err := pipe.New(
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	err = pipe.Wait(playback.Run())
	assert.Nil(t, err)
}

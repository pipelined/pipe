// +build portaudio

package portaudio_test

import (
	"testing"

	"github.com/pipelined/phono/portaudio"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = 512
)

func TestSink(t *testing.T) {
	sink := portaudio.NewSink()

	err := sink.Flush("")
	assert.Nil(t, err)

	fn, err := sink.Sink("", 0, 0, 10)
	assert.Nil(t, fn)
	assert.NotNil(t, err)

	fn, err = sink.Sink("", 44100, 2, 512)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	err = fn([][]float64{{0, 0, 0}})
	assert.Nil(t, err)

	err = sink.Flush("")
	assert.Nil(t, err)
}

package portaudio_test

import (
	"testing"

	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"

	"github.com/dudk/phono"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = phono.BufferSize(512)
	inFile     = "../_testdata/sample1.wav"
)

func TestSink(t *testing.T) {
	if mock.SkipPortaudio {
		t.Skip("Skip portaudio_test.TestSink")
	}
	pump, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	sampleRate := pump.WavSampleRate()
	sink := portaudio.NewSink(bufferSize, pump.WavSampleRate(), pump.WavNumChannels())
	assert.Nil(t, err)

	playback := pipe.New(
		sampleRate,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)

	err = playback.Do(pipe.Run)
	assert.Nil(t, err)
}

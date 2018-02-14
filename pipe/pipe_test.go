package pipe

import (
	"context"
	"testing"

	"github.com/dudk/phono/pipe/pump"
	"github.com/dudk/phono/pipe/sink"
	"github.com/stretchr/testify/assert"
)

var (
	inFile         = "../_testdata/test.wav"
	outFile        = "../_testdata/out.wav"
	bufferSize     = 512
	sampleRate     = 44100
	bitDepth       = 16
	numChannels    = 2
	wavAudioFormat = 1
)

func TestPipe(t *testing.T) {
	pump := pump.NewWav(inFile, bufferSize)
	sink := sink.NewWav(outFile, bufferSize, sampleRate, bitDepth, numChannels, wavAudioFormat)
	pipe := New(
		Pump(*pump),
		Sinks(*sink),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err := pipe.Run(ctx)
	assert.Nil(t, err)
}

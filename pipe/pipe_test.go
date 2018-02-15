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
	outFile2       = "../_testdata/out2.wav"
	bufferSize     = 512
	sampleRate     = 44100
	bitDepth       = 16
	numChannels    = 2
	wavAudioFormat = 1
)

func TestPipe(t *testing.T) {
	pump := pump.NewWav(inFile, bufferSize)
	wavSink := sink.NewWav(outFile, bufferSize, sampleRate, bitDepth, numChannels, wavAudioFormat)
	wavSink1 := sink.NewWav(outFile2, bufferSize, sampleRate, bitDepth, numChannels, wavAudioFormat)
	pipe := New(
		Pump(*pump),
		Sinks(*wavSink, *wavSink1),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err := pipe.Run(ctx)
	assert.Nil(t, err)
}

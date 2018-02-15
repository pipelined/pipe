package sink

import (
	"context"
	"fmt"
	"testing"

	"github.com/dudk/phono/pipe/pump"
	"github.com/stretchr/testify/assert"
)

var (
	inFile     = "../../_testdata/test.wav"
	outFile    = "../../_testdata/out.wav"
	bufferSize = 512
	// sampleRate     = 44100
	// bitDepth       = 16
	// numChannels    = 2
	// wavAudioFormat = 1
)

func TestWavSink(t *testing.T) {

	pump, err := pump.NewWav(inFile, bufferSize)
	assert.Nil(t, err)
	sink := NewWav(outFile, bufferSize, pump.SampleRate, pump.BitDepth, pump.NumChannels, pump.WavAudioFormat)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, _, err := pump.Pump(ctx)
	assert.Nil(t, err)

	errorc, err := sink.Sink(ctx, out)
	assert.Nil(t, err)
	for err = range errorc {
		fmt.Printf("Error waiting for sink: %v", err)
	}
	assert.Nil(t, err)
}

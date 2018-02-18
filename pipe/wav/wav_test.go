package wav_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dudk/phono/pipe/wav"
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

func TestWavPump(t *testing.T) {
	reader, err := wav.NewPump("../../_testdata/test.wav", 512)
	assert.Nil(t, err)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, errorc, err := reader.Pump(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 44100, reader.SampleRate)
	assert.Equal(t, 16, reader.BitDepth)
	assert.Equal(t, 2, reader.NumChannels)
	samplesRead, bufCount := 0, 0
	for out != nil {
		select {
		case buf, ok := <-out:
			if !ok {
				out = nil
			} else {
				samplesRead = samplesRead + buf.Size()
				bufCount++
			}
		case err = <-errorc:
			assert.Nil(t, err)
		}

	}
	assert.Equal(t, 646, bufCount)
	assert.Equal(t, 330534, samplesRead)
}

func TestWavSink(t *testing.T) {

	pump, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	sink := wav.NewSink(outFile, bufferSize, pump.SampleRate, pump.BitDepth, pump.NumChannels, pump.WavAudioFormat)

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

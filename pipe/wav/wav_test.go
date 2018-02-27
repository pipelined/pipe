package wav_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dudk/phono/session"

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
	p, err := wav.NewPump("../../_testdata/test.wav", bufferSize)
	assert.Nil(t, err)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	s := session.New(
		session.SampleRate(p.SampleRate),
		session.NumChannels(p.NumChannels),
		session.BufferSize(p.BufferSize),
	)
	pump := p.Pump(s)
	out, errorc, err := pump(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 44100, p.SampleRate)
	assert.Equal(t, 16, p.BitDepth)
	assert.Equal(t, 2, p.NumChannels)

	samplesRead, bufCount := 0, 0
	for out != nil {
		select {
		case m, ok := <-out:
			if !ok {
				out = nil
			} else {
				samplesRead = samplesRead + m.BufferLen()
				bufCount++
			}
		case err = <-errorc:
			assert.Nil(t, err)
		}

	}
	assert.Equal(t, 646, bufCount)
	assert.Equal(t, 330534, samplesRead/p.NumChannels)
}

func TestWavSink(t *testing.T) {

	p, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)

	session := session.New(
		session.SampleRate(p.SampleRate),
		session.NumChannels(p.NumChannels),
		session.BufferSize(p.BufferSize),
	)
	pump := p.Pump(session)
	s := wav.NewSink(outFile, bufferSize, p.SampleRate, p.BitDepth, p.NumChannels, p.WavAudioFormat)
	s.SetSession(session)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, _, err := pump(ctx)
	assert.Nil(t, err)

	errorc, err := s.Sink(ctx, out)
	assert.Nil(t, err)
	for err = range errorc {
		fmt.Printf("Error waiting for sink: %v", err)
	}
	assert.Nil(t, err)
}

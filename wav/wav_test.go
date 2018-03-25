package wav_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dudk/phono"
	"github.com/dudk/phono/session"

	"github.com/dudk/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	inFile     = "../_testdata/test.wav"
	outFile    = "../_testdata/out.wav"
	bufferSize = 512
	// sampleRate     = 44100
	// bitDepth       = 16
	// numChannels    = 2
	// wavAudioFormat = 1
)

func TestWavPump(t *testing.T) {
	p, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	s := session.New(
		session.SampleRate(p.SampleRate),
		session.NumChannels(p.NumChannels),
		session.BufferSize(bufferSize),
	)

	pulse := s.Pulse()
	pc := make(chan phono.Pulse)
	defer close(pc)

	pump := p.Pump(pulse)
	out, errorc, err := pump(ctx, pc)
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
				samplesRead = samplesRead + m.BufferSize()*p.NumChannels
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
	)
	pulse := session.Pulse()
	pc := make(chan phono.Pulse)
	defer close(pc)

	s := wav.NewSink(outFile, p.BitDepth, p.WavAudioFormat)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, _, err := p.Pump(pulse)(ctx, pc)
	assert.Nil(t, err)

	errorc, err := s.Sink(pulse)(ctx, out)
	assert.Nil(t, err)
	for err = range errorc {
		fmt.Printf("Error waiting for sink: %v", err)
	}
	assert.Nil(t, err)
}

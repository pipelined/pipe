package wav_test

import (
	"fmt"
	"testing"

	"github.com/go-audio/audio"

	"github.com/dudk/wav"
	"github.com/stretchr/testify/assert"
)

var (
	inFile     = "_testdata/test.wav"
	outFile    = "_testdata/out.wav"
	bufferSize = 512
)

// TODO: build with local mock package

func TestWavPump(t *testing.T) {
	// p, err := wav.NewPump(inFile, bufferSize)
	// assert.Nil(t, err)
	// ctx, cancelFunc := context.WithCancel(context.Background())
	// defer cancelFunc()

	// s := session.New(
	// 	session.SampleRate(p.SampleRate),
	// 	session.NumChannels(p.NumChannels),
	// 	session.BufferSize(bufferSize),
	// )

	// pulse := s.Pulse()
	// pc := make(chan phono.Pulse)
	// defer close(pc)

	// pump := p.Pump(pulse)
	// out, errorc, err := pump(ctx, pc)
	// assert.Nil(t, err)
	// assert.Equal(t, 44100, p.SampleRate)
	// assert.Equal(t, 16, p.BitDepth)
	// assert.Equal(t, 2, p.NumChannels)

	// samplesRead, bufCount := 0, 0
	// for out != nil {
	// 	select {
	// 	case m, ok := <-out:
	// 		if !ok {
	// 			out = nil
	// 		} else {
	// 			samplesRead = samplesRead + m.BufferSize()*m.NumChannels()
	// 			bufCount++
	// 		}
	// 	case err = <-errorc:
	// 		assert.Nil(t, err)
	// 	}

	// }
	// assert.Equal(t, 646, bufCount)
	// assert.Equal(t, 330534, samplesRead/p.NumChannels)
}

func TestWavSink(t *testing.T) {

	// p, err := wav.NewPump(inFile, bufferSize)
	// assert.Nil(t, err)

	// session := session.New(
	// 	session.SampleRate(p.SampleRate),
	// 	session.NumChannels(p.NumChannels),
	// )
	// pulse := session.Pulse()
	// pc := make(chan phono.Pulse)
	// defer close(pc)

	// s := wav.NewSink(outFile, p.BitDepth, p.WavAudioFormat)
	// ctx, cancelFunc := context.WithCancel(context.Background())
	// defer cancelFunc()
	// out, _, err := p.Pump(pulse)(ctx, pc)
	// assert.Nil(t, err)

	// errorc, err := s.Sink(pulse)(ctx, out)
	// assert.Nil(t, err)
	// for err = range errorc {
	// 	fmt.Printf("Error waiting for sink: %v", err)
	// }
	// assert.Nil(t, err)
}

func TestIntBufferToSamples(t *testing.T) {
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 2,
			SampleRate:  44100,
		},
		Data: []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
	}
	samples, err := wav.AsSamples(buf)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(samples))
	assert.Equal(t, 8, len(samples[0]))
	for _, v := range samples[0] {
		assert.Equal(t, float64(1)/0x8000, v)
	}

	_, err = wav.AsSamples(nil)
	assert.Nil(t, err)
	buf.Format = nil
	_, err = wav.AsSamples(buf)
	assert.EqualError(t, err, "Format for Buffer is not defined")
	pcmbuf := &audio.PCMBuffer{Format: &audio.Format{}}
	_, err = wav.AsSamples(pcmbuf)
	assert.EqualError(t, err, fmt.Sprintf("Conversion to [][]float64 from %T is not defined", pcmbuf))
}

func TestSampelsToIntBuffer(t *testing.T) {
	ib := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 2,
			SampleRate:  44100,
		},
	}
	samples := [][]float64{
		[]float64{1, 1, 1, 1, 1, 1, 1, 1},
		[]float64{2, 2, 2, 2, 2, 2, 2, 2},
	}
	err := wav.AsBuffer(ib, samples)
	assert.Nil(t, err)
	assert.NotNil(t, ib)
	assert.Equal(t, 8, ib.NumFrames())
	for i := 0; i < len(ib.Data); i = i + 2 {
		assert.Equal(t, 0x7fff, ib.Data[i])
	}
	err = wav.AsBuffer(nil, nil)
	assert.Nil(t, err)
}

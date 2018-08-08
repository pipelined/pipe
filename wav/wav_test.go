package wav_test

import (
	"fmt"
	"testing"

	"github.com/dudk/phono/pipe/runner"

	"github.com/dudk/phono/mock"

	"github.com/go-audio/audio"

	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/wav"
	"github.com/stretchr/testify/assert"
)

var (
	bufferSize = phono.BufferSize(512)
)

var tests = []struct {
	phono.BufferSize
	inFile   string
	outFile  string
	messages int64
	samples  int64
}{
	{
		BufferSize: 512,
		inFile:     "../_testdata/sample1.wav",
		outFile:    "../_testdata/out/wav1.wav",
		messages:   646,
		samples:    330534,
	},
	{
		BufferSize: 512,
		inFile:     "../_testdata/out/wav1.wav",
		outFile:    "../_testdata/out/wav2.wav",
		messages:   646,
		samples:    330534,
	},
}

func TestWavPipe(t *testing.T) {
	for _, test := range tests {
		pump, err := wav.NewPump(test.inFile, bufferSize)
		assert.Nil(t, err)
		sink, err := wav.NewSink(test.outFile, pump.WavSampleRate(), pump.WavNumChannels(), pump.WavBitDepth(), pump.WavAudioFormat())
		assert.Nil(t, err)

		processor := &mock.Processor{}
		p := pipe.New(
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		err = p.Do(pipe.Run)
		assert.Nil(t, err)
		messageCount, sampleCount := processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, sampleCount)

		err = p.Do(pipe.Run)
		assert.Equal(t, runner.ErrSingleUseReused, err)
	}
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
	assert.Equal(t, phono.NumChannels(2), samples.NumChannels())
	assert.Equal(t, phono.BufferSize(8), samples.Size())
	for _, v := range (samples)[0] {
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

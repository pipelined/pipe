package phono_test

import (
	"fmt"
	"testing"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/stretchr/testify/assert"
)

func TestIntBufferToSamples(t *testing.T) {
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 2,
			SampleRate:  44100,
		},
		Data: []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
	}
	samples, err := phono.AsSamples(buf)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(samples))
	assert.Equal(t, 8, len(samples[0]))
	for _, v := range samples[0] {
		assert.Equal(t, float64(1)/0x8000, v)
	}

	_, err = phono.AsSamples(nil)
	assert.Nil(t, err)
	buf.Format = nil
	_, err = phono.AsSamples(buf)
	assert.EqualError(t, err, "Format for Buffer is not defined")
	pcmbuf := &audio.PCMBuffer{Format: &audio.Format{}}
	_, err = phono.AsSamples(pcmbuf)
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
	err := phono.AsBuffer(ib, samples)
	assert.Nil(t, err)
	assert.NotNil(t, ib)
	assert.Equal(t, 8, ib.NumFrames())
	for i := 0; i < len(ib.Data); i = i + 2 {
		assert.Equal(t, 0x7fff, ib.Data[i])
	}
	err = phono.AsBuffer(nil, nil)
	assert.Nil(t, err)
}

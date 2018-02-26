package session_test

import (
	"testing"

	"github.com/dudk/phono/internal/session"
	"github.com/go-audio/audio"
	"github.com/stretchr/testify/assert"
)

func TestBufferFromSampels(t *testing.T) {
	s := session.New(
		session.BufferSize(8),
		session.SampleRate(44100),
		session.NumChannels(2),
	)
	m := s.NewMessage()
	assert.Equal(t, 16, m.BufferLen())
	samples := [][]float64{
		[]float64{1, 1, 1, 1, 1, 1, 1, 1},
		[]float64{2, 2, 2, 2, 2, 2, 2, 2},
	}
	m.PutSamples(samples)
	buf := m.AsBuffer()
	assert.Equal(t, 8, buf.NumFrames())
}

func TestSamplesFromBuffer(t *testing.T) {
	s := session.New(
		session.BufferSize(8),
		session.SampleRate(44100),
		session.NumChannels(2),
	)
	m := s.NewMessage()
	assert.Equal(t, 16, m.BufferLen())
	buf := &audio.FloatBuffer{
		Format: &audio.Format{
			NumChannels: 2,
			SampleRate:  44100,
		},
		Data: []float64{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
	}
	m.PutBuffer(buf, len(buf.Data))
	samples := m.AsSamples()
	assert.Equal(t, 2, len(samples))
	assert.Equal(t, 8, len(samples[0]))
	for _, v := range samples[0] {
		assert.Equal(t, float64(1), v)
	}
}

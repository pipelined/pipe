package session_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono/session"
)

var (
	bufferSize  = 512
	sampleRate  = 44100
	numChannels = 2
)

func TestSession(t *testing.T) {
	s := session.New(
		session.SampleRate(sampleRate),
		session.BufferSize(bufferSize),
		session.NumChannels(numChannels),
	)
	assert.NotNil(t, s)
	assert.Equal(t, sampleRate, s.SampleRate())
	assert.Equal(t, bufferSize, s.BufferSize())
	assert.Equal(t, numChannels, s.NumChannels())

	p := s.Pulse()
	assert.NotNil(t, p)
	assert.Equal(t, sampleRate, p.SampleRate())
	assert.Equal(t, bufferSize, p.BufferSize())
	assert.Equal(t, numChannels, p.NumChannels())

	m := p.NewMessage()
	assert.NotNil(t, m)

	pnew := m.Pulse()
	assert.Nil(t, pnew)

	m.SetPulse(p)
	pnew = m.Pulse()
	assert.NotNil(t, pnew)
	assert.Equal(t, p.SampleRate(), pnew.SampleRate())
	assert.Equal(t, p.BufferSize(), pnew.BufferSize())
	assert.Equal(t, p.NumChannels(), pnew.NumChannels())
}

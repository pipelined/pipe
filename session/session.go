package session

import (
	"github.com/dudk/phono"
)

// Session is a top level abstraction
type Session struct {
	sampleRate  int
	bufferSize  int
	numChannels int
}

// New creates a new session
func New(options ...Option) *Session {
	s := &Session{}
	for _, option := range options {
		option(s)
	}
	return s
}

// SampleRate returns session's sample rate
func (s Session) SampleRate() int {
	return s.sampleRate
}

// BufferSize returns session's buffer size
func (s Session) BufferSize() int {
	return s.bufferSize
}

// NumChannels returns session's number of channels
// this is a temporary option, for session channels different model is required
func (s Session) NumChannels() int {
	return s.numChannels
}

// Option of a session
type Option func(s *Session) Option

// SampleRate defines sample rate
func SampleRate(sampleRate int) Option {
	return func(s *Session) Option {
		previous := s.sampleRate
		s.sampleRate = sampleRate
		return SampleRate(previous)
	}
}

// BufferSize defines sample rate
func BufferSize(bufferSize int) Option {
	return func(s *Session) Option {
		previous := s.bufferSize
		s.bufferSize = bufferSize
		return BufferSize(previous)
	}
}

// NumChannels defines num channels for session
func NumChannels(numChannels int) Option {
	return func(s *Session) Option {
		previous := s.numChannels
		s.numChannels = numChannels
		return BufferSize(previous)
	}
}

// NewMessage implements pipe.Session interface
func (s Session) NewMessage() phono.Message {
	return &Message{
		bufferLen: s.numChannels * s.bufferSize,
	}
}

// Message is a DTO for pipe
type Message struct {
	samples   [][]float64
	bufferLen int
}

// IsEmpty returns true if buffer and samples are empty
func (m *Message) IsEmpty() bool {
	if m.samples == nil || m.samples[0] == nil {
		return true
	}
	return false
}

// BufferLen returns underlying buffer len
func (m *Message) BufferLen() int {
	return m.bufferLen
}

// PutSamples assignes samples to message
// and also set buffer data to nil
func (m *Message) PutSamples(s [][]float64) {
	if s != nil {
		// we need to adjust buffer to our samples
		m.samples = s
		m.bufferLen = len(s) * len(s[0])
	}
}

// Size returns length of samples per channel
func (m *Message) Size() int {
	if m.IsEmpty() {
		return 0
	}
	return len(m.samples[0])
}

// Samples returns message samples
// if needed, data pulled from buffer
// receiver is not pointer because we need a copy
func (m *Message) Samples() (out [][]float64) {
	if m == nil {
		return nil
	}
	return m.samples
}

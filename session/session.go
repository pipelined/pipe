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

// Track is a sequence of pipes which are executed one after another
type Track struct {
	pipes map[phono.SamplePosition]phono.Pipe
}

// Message is a DTO for pipe
type Message struct {
	bufferSize  int
	numChannels int
	samples     [][]float64
	pulse       phono.Pulse
}

// Pulse represents audio properties of the message
// it's getting sent once properties changed
type Pulse struct {
	tempo                    float64
	timeSignatureNumerator   int
	timeSignatureDenominator int
	bufferSize               int
	numChannels              int
	sampleRate               int
}

// Tempo returns tempo value
func (p Pulse) Tempo() float64 {
	return p.tempo
}

// TimeSignature returns musical time signature i.e: 4/4 or 3/4
func (p Pulse) TimeSignature() (int, int) {
	return p.timeSignatureNumerator, p.timeSignatureDenominator
}

// NewMessage produces new message for pipe, with attributes defined for passed SamplePosition
func (p Pulse) NewMessage() phono.Message {
	return &Message{
		bufferSize: p.bufferSize,
	}
}

// BufferSize returns buffer size
func (p Pulse) BufferSize() int {
	return p.bufferSize
}

// SampleRate returns sample rate
func (p Pulse) SampleRate() int {
	return p.sampleRate
}

// NumChannels returns number of channels
func (p Pulse) NumChannels() int {
	return p.numChannels
}

// New creates a new session
func New(options ...Option) *Session {
	s := &Session{}
	for _, option := range options {
		option(s)
	}
	return s
}

// Pulse returns current pulse
func (s Session) Pulse() phono.Pulse {
	return Pulse{
		sampleRate:  s.sampleRate,
		numChannels: s.numChannels,
		bufferSize:  s.bufferSize,
	}
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

// BufferSize returns underlying buffer len
func (m *Message) BufferSize() int {
	return m.bufferSize
}

// NumChannels returns underlying buffer len
func (m *Message) NumChannels() int {
	return m.numChannels
}

// SetSamples assignes samples to message
// and also set buffer data to nil
func (m *Message) SetSamples(s [][]float64) {
	m.bufferSize = 0
	m.numChannels = 0
	if s != nil {
		m.samples = s
		m.numChannels = len(s)
		if s[0] != nil {
			m.bufferSize = len(s[0])
		}
	}
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

// SetPulse assigns pulse to message
func (m *Message) SetPulse(p phono.Pulse) {
	m.pulse = p
}

// Pulse returns pulse of message
func (m *Message) Pulse() phono.Pulse {
	return m.pulse
}

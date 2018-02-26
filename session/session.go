package session

import (
	"github.com/dudk/phono"
	"github.com/go-audio/audio"
)

// Session is a top level abstraction
type Session struct {
	SampleRate  int
	BufferSize  int
	NumChannels int
}

// New creates a new session
func New(options ...Option) *Session {
	s := &Session{}
	for _, option := range options {
		option(s)
	}
	return s
}

// Option of a session
type Option func(s *Session) Option

// SampleRate defines sample rate
func SampleRate(sampleRate int) Option {
	return func(s *Session) Option {
		previous := s.SampleRate
		s.SampleRate = sampleRate
		return SampleRate(previous)
	}
}

// BufferSize defines sample rate
func BufferSize(bufferSize int) Option {
	return func(s *Session) Option {
		previous := s.BufferSize
		s.BufferSize = bufferSize
		return BufferSize(previous)
	}
}

// NumChannels defines num channels for session
func NumChannels(numChannels int) Option {
	return func(s *Session) Option {
		previous := s.NumChannels
		s.NumChannels = numChannels
		return BufferSize(previous)
	}
}

// NewMessage implements pipe.Session interface
func (s *Session) NewMessage() phono.Message {
	return &Message{
		format: &audio.Format{
			NumChannels: s.NumChannels,
			SampleRate:  s.SampleRate,
		},
		bufferLen: s.NumChannels * s.BufferSize,
	}
}

// Message is a DTO for pipe
type Message struct {
	samples   [][]float64
	buffer    audio.Buffer
	bufferLen int
	format    *audio.Format
}

// NewMessage constructs a new message with defined properties
func NewMessage(bufferSize int, numChannels int, sampleRate int) *Message {
	return &Message{
		format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		bufferLen: numChannels * bufferSize,
	}
}

// IsEmpty returns true if buffer and samples are empty
func (m *Message) IsEmpty() bool {
	if m.samples == nil && (m.buffer == nil || m.buffer.NumFrames() == 0) {
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
		m.buffer = nil
		m.samples = s
		m.format.NumChannels = len(s)
		m.bufferLen = len(s) * len(s[0])
	}
}

// PutBuffer assignes pcm buffer to message
// and also set samples to nil
func (m *Message) PutBuffer(buffer audio.Buffer, bufferLen int) {
	if buffer != nil {
		m.samples = nil
		m.buffer = buffer
		m.format = buffer.PCMFormat()
		m.bufferLen = bufferLen
	}
}

// Size returns length of samples per channel
func (m *Message) Size() int {
	if m.IsEmpty() {
		return 0
	}
	if m.samples != nil && m.samples[0] != nil {
		return len(m.samples[0])
	}

	return m.buffer.NumFrames()
}

// AsSamples returns message as samples
// if needed, data pulled from buffer
func (m *Message) AsSamples() [][]float64 {
	if m.IsEmpty() {
		return nil
	}
	if m.samples != nil {
		return m.samples
	}
	numChannels := m.format.NumChannels
	// convert buffer to float buffer
	floatBuf := m.buffer.AsFloatBuffer()
	m.samples = make([][]float64, numChannels)
	for i := range m.samples {
		m.samples[i] = make([]float64, 0, floatBuf.NumFrames())
		for j := i; j < m.bufferLen; j = j + numChannels {
			m.samples[i] = append(m.samples[i], floatBuf.Data[j])
		}
	}
	return m.samples
}

// AsBuffer returns message as audio.FloatBuffer
// if needed, data pulled samples
func (m *Message) AsBuffer() audio.Buffer {
	if m.IsEmpty() {
		return nil
	}
	if m.buffer != nil {
		return m.buffer
	}
	numChannels := m.format.NumChannels
	buf := &audio.FloatBuffer{
		Format: m.format,
		Data:   make([]float64, m.bufferLen),
	}
	for i := range m.samples[0] {
		for j := range m.samples {
			buf.Data[i*numChannels+j] = m.samples[j][i]
		}
	}
	m.buffer = buf
	return buf
}

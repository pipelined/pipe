package phono

import (
	"github.com/go-audio/audio"
)

// Samples are used to represent samples data in
// two-dimensional array where first dimension if for channels
type Samples [][]float64

// Size returns a size of buffer
func (s Samples) Size() int {
	if s == nil || len(s) == 0 || s[0] == nil {
		return 0
	}
	return len(s[0])
}

// Message is a DTO for pipe
type Message struct {
	samples   Samples
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
func (m *Message) PutSamples(s Samples) {
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
	return m.samples.Size()
}

// AsSamples returns message as samples
// if needed, data pulled from buffer
func (m *Message) AsSamples() Samples {
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

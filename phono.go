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
	buffer    *audio.FloatBuffer
	bufferLen int
}

// NewMessage constructs a new message with defined properties
func NewMessage(bufferSize int, numChannels int, sampleRate int) *Message {
	bufferLen := numChannels * bufferSize
	return &Message{
		buffer: &audio.FloatBuffer{
			Format: &audio.Format{
				NumChannels: numChannels,
				SampleRate:  sampleRate,
			},
		},
		bufferLen: bufferLen,
	}
}

// IsEmpty returns true if buffer and samples are empty
func (m *Message) IsEmpty() bool {
	if m.samples == nil && (m.buffer.Data == nil || len(m.buffer.Data) == 0) {
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
		m.samples = s
		// if buffer was assigned, we need to clear it, so only format is copied
		m.buffer.Data = nil
	}
}

// PutBuffer assignes pcm buffer to message
// and also set samples to nil
func (m *Message) PutBuffer(buffer audio.Buffer, bufferLen int) {
	if buffer != nil {
		m.buffer = buffer.AsFloatBuffer()
		m.bufferLen = bufferLen
		m.samples = nil
	}
}

// Samples returns messages samples
// func (m *Message) Samples() Samples {
// 	return m.samples
// }

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
	numChannels := m.buffer.PCMFormat().NumChannels
	samples := make([][]float64, numChannels)
	for i := range samples {
		samples[i] = make([]float64, 0, m.buffer.NumFrames())
		for j := i; j < m.bufferLen; j = j + numChannels {
			samples[i] = append(samples[i], m.buffer.Data[j])
		}
	}
	m.PutSamples(samples)
	return m.samples
}

// AsBuffer returns message as audio.FloatBuffer
// if needed, data pulled samples
func (m *Message) AsBuffer() *audio.FloatBuffer {
	if m.IsEmpty() {
		return nil
	}
	if m.buffer.Data != nil {
		return m.buffer
	}
	numChannels := m.buffer.Format.NumChannels
	m.buffer.Data = make([]float64, m.bufferLen)
	for i := range m.samples[0] {
		for j := range m.samples {
			m.buffer.Data[i*numChannels+j] = m.samples[j][i]
		}
	}
	m.PutBuffer(m.buffer, m.bufferLen)
	return m.buffer
}

func convertToWavBuffer(s Samples, sampleRate int, bitDepth int) *audio.IntBuffer {

	bufLen := len(s) * len(s[0])
	numChannels := len(s)
	intBuffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
		Data:           make([]int, bufLen),
	}

	for i := range s[0] {
		for j := range s {
			intBuffer.Data[i*numChannels+j] = int(s[j][i] * 0x7FFF)
		}
	}
	return &intBuffer
}

func convertFromWavBuffer(intBuffer *audio.IntBuffer, wavLen int) Samples {
	if intBuffer == nil {
		return nil
	}
	s := Samples{}
	numChannels := intBuffer.Format.NumChannels
	s = make([][]float64, numChannels)

	size := wavLen / numChannels
	for i := range s {
		s[i] = make([]float64, 0, size)
		for j := i; j < wavLen; j = j + numChannels {
			s[i] = append(s[i], float64(intBuffer.Data[j])/0x8000)
		}
	}
	return s
}

package phono

//Buffer is used to pass data across application
type Buffer struct {
	Samples [][]float64
}

//Size returns a size of buffer
func (b *Buffer) Size() int {
	if b.Samples == nil || len(b.Samples) == 0 || b.Samples[0] == nil {
		return 0
	}
	return len(b.Samples[0])
}

//Message is a DTO for pipe
type Message struct {
	samples *Buffer
}

//PutSamples assignes samples to message
func (m *Message) PutSamples(buf *Buffer) {
	m.samples = buf
}

//Samples returns messages samples
func (m *Message) Samples() *Buffer {
	return m.samples
}

//Size returns length of samples per channel
func (m *Message) Size() int {
	return m.samples.Size()
}

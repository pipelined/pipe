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

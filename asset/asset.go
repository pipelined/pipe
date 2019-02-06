package asset

import (
	"github.com/pipelined/phono/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used as processing input and always should be copied.
type Asset struct {
	buffer signal.Float64
}

// Sink appends buffers to asset.
func (a *Asset) Sink(sourceID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	return func(b [][]float64) error {
		a.buffer = a.buffer.Append(b)
		return nil
	}, nil
}

// Asset returns asset's buffer
func (a *Asset) Asset() signal.Float64 {
	if a != nil {
		return a.buffer
	}
	return nil
}

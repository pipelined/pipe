package audio

import (
	"github.com/pipelined/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used as processing input and always should be copied.
type Asset struct {
	data signal.Float64
}

// NewAsset creates new asset from signal.Float64 buffer.
func NewAsset(floats signal.Float64) *Asset {
	return &Asset{data: floats}
}

// Sink appends buffers to asset.
func (a *Asset) Sink(sourceID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	return func(b [][]float64) error {
		a.data = a.data.Append(b)
		return nil
	}, nil
}

// Data returns asset's data.
func (a *Asset) Data() signal.Float64 {
	if a == nil {
		return nil
	}
	return a.data
}

// NumChannels returns a number of channels of the asset data.
func (a *Asset) NumChannels() int {
	if a == nil || a.data == nil {
		return 0
	}
	return a.data.NumChannels()
}

// Clip represents a segment of an asset.
//
// Clip refers to an asset, but it's not a copy.
type Clip struct {
	*Asset
	Start int
	Len   int
}

// Clip creates a new clip from buffer with defined start and length.
//
// if start >= buffer size, nil is returned
// if start + len >= buffer size, len is decreased till the end of slice
// if start < 0, nil is returned
func (a *Asset) Clip(start int, len int) Clip {
	size := a.data.Size()
	if a.data == nil || start >= size || start < 0 {
		return Clip{}
	}
	end := start + len
	if end >= size {
		len = size - start
	}
	return Clip{
		Asset: a,
		Start: start,
		Len:   len,
	}
}

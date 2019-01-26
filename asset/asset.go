package asset

import (
	"sync"

	"github.com/pipelined/phono"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used as processing input and always should be copied.
type Asset struct {
	phono.Buffer

	once sync.Once
}

// New creates asset.
func New() *Asset {
	return &Asset{}
}

// Reset implements pipe.Resetter
func (a *Asset) Reset(string) error {
	return phono.SingleUse(&a.once)
}

// Sink appends buffers to asset.
func (a *Asset) Sink(string) (func(phono.Buffer) error, error) {
	return func(b phono.Buffer) error {
		a.Buffer = a.Buffer.Append(b)
		return nil
	}, nil
}

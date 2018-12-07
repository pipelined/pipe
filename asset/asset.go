package asset

import (
	"sync"

	"github.com/dudk/phono"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used as processing input and always should be copied.
type Asset struct {
	phono.UID
	phono.Buffer

	once sync.Once
}

// New creates asset.
func New() *Asset {
	return &Asset{
		UID: phono.NewUID(),
	}
}

// Reset implements pipe.Resetter
func (a *Asset) Reset(string) error {
	return phono.SingleUse(&a.once)
}

// Sink appends buffers to asset.
func (a *Asset) Sink(string) (phono.SinkFunc, error) {
	return func(b phono.Buffer) error {
		a.Buffer = a.Buffer.Append(b)
		return nil
	}, nil
}

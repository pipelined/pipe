package asset

import (
	"sync"

	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used as processing input and always should be copied.
type Asset struct {
	phono.UID
	phono.SampleRate
	phono.Buffer

	once sync.Once
}

// Sink appends buffers to asset.
func (a *Asset) Sink(string) (phono.SinkFunc, error) {
	err := pipe.SingleUse(&a.once)
	if err != nil {
		return nil, err
	}
	return func(b phono.Buffer) error {
		a.Buffer = a.Buffer.Append(b)
		return nil
	}, nil
}

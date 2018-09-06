package asset

import (
	"sync"

	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/pipe/runner"
)

// Sink is a sink which uses a regular buffer as underlying storage
// it can be used as processing input and always should be copied
type Sink struct {
	phono.UID
	phono.SampleRate
	phono.Buffer

	once sync.Once
}

// RunSink returns initialised runner for asset sink
func (a *Sink) RunSink(string) pipe.SinkRunner {
	return &runner.Sink{
		Sink: a,
		Before: func() error {
			return runner.SingleUse(&a.once)
		},
	}
}

// Sink appends buffers to asset
func (a *Sink) Sink(m *phono.Message) error {
	a.Buffer = a.Buffer.Append(m.Buffer)
	return nil
}

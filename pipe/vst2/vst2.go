package vst2

import (
	"context"

	"github.com/dudk/phono"
	"github.com/dudk/phono/vst2"
)

// Processor represents vst2 sound processor
type Processor struct {
	session    phono.Session
	plugin     *vst2.Plugin
	BufferSize int
	SampleRate int
}

// NewProcessor creates new vst2 processor
func NewProcessor(plugin *vst2.Plugin, bufferSize int, sampleRate int) *Processor {
	return &Processor{
		plugin:     plugin,
		SampleRate: sampleRate,
		BufferSize: bufferSize,
	}
}

// SetSession assigns a session to the pump
func (p *Processor) SetSession(s phono.Session) {
	p.session = s
}

// Process implements processor.Processor
func (p *Processor) Process(ctx context.Context, in <-chan phono.Message) (<-chan phono.Message, <-chan error, error) {
	errc := make(chan error, 1)
	out := make(chan phono.Message)
	go func() {
		defer close(out)
		defer close(errc)
		p.plugin.BufferSize(p.BufferSize)
		p.plugin.SampleRate(p.SampleRate)
		p.plugin.Resume()
		defer p.plugin.Suspend()
		for in != nil {
			select {
			case message, ok := <-in:
				if !ok {
					in = nil
				} else {
					message.PutSamples(p.plugin.Process(message.AsSamples()))
					out <- message
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errc, nil
}

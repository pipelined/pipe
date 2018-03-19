package vst2

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/dudk/phono"
	"github.com/dudk/phono/vst2"
)

// Processor represents vst2 sound processor
type Processor struct {
	plugin *vst2.Plugin
}

// NewProcessor creates new vst2 processor
func NewProcessor(plugin *vst2.Plugin) *Processor {
	return &Processor{
		plugin: plugin,
	}
}

// Process implements processor.Processor
func (p *Processor) Process(s phono.Session) phono.ProcessFunc {
	p.plugin.SetCallback(callback(s))
	return func(ctx context.Context, in <-chan phono.Message) (<-chan phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		out := make(chan phono.Message)
		go func() {
			defer close(out)
			defer close(errc)
			p.plugin.BufferSize(s.BufferSize())
			p.plugin.SampleRate(s.SampleRate())
			p.plugin.SetSpeakerArrangement(2)
			p.plugin.Resume()
			defer p.plugin.Suspend()
			for in != nil {
				select {
				case message, ok := <-in:
					if !ok {
						in = nil
					} else {
						samples := message.Samples()
						processed := p.plugin.Process(samples)
						message.PutSamples(processed)
						out <- message
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, errc, nil
	}
}

func callback(s phono.Session) vst2.HostCallbackFuncAlias {
	return func(p *vst2.PluginAlias, opcode vst2.MasterOpcodeAlias, index int64, value int64, ptr unsafe.Pointer, opt float64) int {
		fmt.Printf("Printf from pipe closure callback! Plugin name: %v\n", p.Name)
		return 0
	}
}

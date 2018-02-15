package processor

import (
	"context"

	"github.com/dudk/phono"
	vst2 "github.com/dudk/vst2"
)

//Vst2 represents vst2 sound processor
type Vst2 struct {
	plugin *vst2.Plugin
}

//NewVst2 creates new vst2 processor
func NewVst2(plugin *vst2.Plugin) *Vst2 {
	return &Vst2{plugin: plugin}
}

//Process implements processor.Processor
func (v Vst2) Process(ctx context.Context, in <-chan phono.Message) (<-chan phono.Message, <-chan error, error) {
	errc := make(chan error, 1)
	out := make(chan phono.Message)
	go func() {
		defer close(out)
		defer close(errc)
		for in != nil {
			select {
			case message, ok := <-in:
				if !ok {
					in = nil
				} else {
					v.plugin.Process(message.Samples().Samples)
					out <- message
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errc, nil
}

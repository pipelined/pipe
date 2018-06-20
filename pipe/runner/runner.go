package runner

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
)

// Process represents processor's runner
type Process struct {
	pipe.Processor
	Before BeforeAfterFunc
	After  BeforeAfterFunc
	in     <-chan *phono.Message
	out    chan *phono.Message
}

// BeforeAfterFunc represents setup/clean up functions which are executed on Run start and finish
type BeforeAfterFunc func() error

// Run the runner
func (r *Process) Run(in <-chan *phono.Message) (<-chan *phono.Message, <-chan error, error) {
	err := r.Before.call()
	if err != nil {
		return nil, nil, err
	}
	errc := make(chan error, 1)
	r.in = in
	r.out = make(chan *phono.Message)
	go func() {
		defer close(r.out)
		defer close(errc)
		defer func() {
			err := r.After.call()
			if err != nil {
				errc <- err
			}
		}()
		for in != nil {
			select {
			case m, ok := <-in:
				if !ok {
					return
				}
				m.ApplyTo(r.Processor)
				m.RecievedBy(r.Processor)
				m.Buffer = r.Process(m.Buffer)
				r.out <- m
			}
		}
	}()
	return r.out, errc, nil
}

func (fn BeforeAfterFunc) call() error {
	if fn == nil {
		return nil
	}
	return fn()
}

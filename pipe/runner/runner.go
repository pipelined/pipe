package runner

import (
	"context"

	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
)

// Pump represents pump's runner
type Pump struct {
	pipe.Pump
	Before BeforeAfterFunc
	After  BeforeAfterFunc
	out    chan *phono.Message
}

// Process represents processor's runner
type Process struct {
	pipe.Processor
	Before BeforeAfterFunc
	After  BeforeAfterFunc
	in     <-chan *phono.Message
	out    chan *phono.Message
}

// Sink represents sink's runner
type Sink struct {
	pipe.Sink
	Before BeforeAfterFunc
	After  BeforeAfterFunc
	in     <-chan *phono.Message
}

// BeforeAfterFunc represents setup/clean up functions which are executed on Run start and finish
type BeforeAfterFunc func() error

// Run the Pump runner
func (p *Pump) Run(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan *phono.Message, <-chan error, error) {
	err := p.Before.call()
	if err != nil {
		return nil, nil, err
	}
	out := make(chan *phono.Message)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		defer func() {
			err := p.After.call()
			if err != nil {
				errc <- err
			}
		}()
		for {
			m := newMessage()
			m.ApplyTo(p.Pump)
			m, err := p.Pump.Pump(m)
			if err != nil {
				if err != pipe.ErrEOP {
					errc <- err
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				out <- m
			}
		}
	}()
	return out, errc, nil
}

// Run the Processor runner
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
				m, err = r.Process(m)
				if err != nil {
					errc <- err
					return
				}
				r.out <- m
			}
		}
	}()
	return r.out, errc, nil
}

// Run the sink runner
func (s *Sink) Run(in <-chan *phono.Message) (<-chan error, error) {
	err := s.Before.call()
	if err != nil {
		return nil, err
	}
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		defer func() {
			err := s.After.call()
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
				m.Params.ApplyTo(s.Sink)
				err = s.Sink.Sink(m)
				if err != nil {
					errc <- err
					return
				}
			}
		}
	}()

	return errc, nil
}

func (fn BeforeAfterFunc) call() error {
	if fn == nil {
		return nil
	}
	return fn()
}

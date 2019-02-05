package pipe

import (
	"io"
)

// pumpRunner is pump's runner.
type pumpRunner struct {
	Pump
	fn  func() ([][]float64, error)
	out chan message
	hooks
}

// processRunner represents processor's runner.
type processRunner struct {
	Processor
	fn  func([][]float64) ([][]float64, error)
	in  <-chan message
	out chan message
	hooks
}

// sinkRunner represents sink's runner.
type sinkRunner struct {
	Sink
	fn func([][]float64) error
	in <-chan message
	hooks
}

// Flusher defines component that must flushed in the end of execution.
type Flusher interface {
	Flush(string) error
}

// Interrupter defines component that has custom interruption logic.
type Interrupter interface {
	Interrupt(string) error
}

// Resetter defines component that must be resetted before consequent use.
type Resetter interface {
	Reset(string) error
}

// hook represents optional functions for components lyfecycle.
type hook func(string) error

// set of hooks for runners.
type hooks struct {
	flush     hook
	interrupt hook
	reset     hook
}

// bindHooks of component.
func bindHooks(v interface{}) hooks {
	return hooks{
		flush:     flusher(v),
		interrupt: interrupter(v),
		reset:     resetter(v),
	}
}

var do struct{}

// flusher checks if interface implements Flusher and if so, return it.
func flusher(i interface{}) hook {
	if v, ok := i.(Flusher); ok {
		return v.Flush
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func interrupter(i interface{}) hook {
	if v, ok := i.(Interrupter); ok {
		return v.Interrupt
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func resetter(i interface{}) hook {
	if v, ok := i.(Resetter); ok {
		return v.Reset
	}
	return nil
}

// newPumpRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newPumpRunner(pipeID string, bufferSize int, p Pump) (*pumpRunner, int, int, error) {
	fn, sampleRate, numChannels, err := p.Pump(pipeID, bufferSize)
	if err != nil {
		return nil, 0, 0, err
	}
	r := pumpRunner{
		fn:    fn,
		Pump:  p,
		hooks: bindHooks(p),
	}
	return &r, sampleRate, numChannels, nil
}

// run the Pump runner.
func (r *pumpRunner) run(pipeID, componentID string, cancel <-chan struct{}, provide chan<- struct{}, consume <-chan message, meter *meter) (<-chan message, <-chan error) {
	out := make(chan message)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)
		call(r.reset, pipeID, errc) // reset hook
		var err error
		var m message
		var done bool // done flag
		for {
			// request new message
			select {
			case provide <- do:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-consume:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.applyTo(componentID) // apply params
			m.buffer, err = r.fn() // pump new buffer
			if err != nil {
				switch err {
				case io.EOF:
					call(r.flush, pipeID, errc) // flush hook
					return
				case io.ErrUnexpectedEOF:
					call(r.flush, pipeID, errc) // flush hook
					done = true
				default:
					errc <- err
					return
				}
			}

			meter = meter.sample(int64(m.buffer.Size())).message()
			m.feedback.applyTo(componentID) // apply feedback

			// push message further
			select {
			case out <- m:
				if done {
					return
				}
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// newProcessRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newProcessRunner(pipeID string, sampleRate, numChannels, bufferSize int, p Processor) (*processRunner, error) {
	fn, err := p.Process(pipeID, sampleRate, numChannels, bufferSize)
	if err != nil {
		return nil, err
	}
	r := processRunner{
		fn:        fn,
		Processor: p,
		hooks:     bindHooks(p),
	}
	return &r, nil
}

// run the Processor runner.
func (r *processRunner) run(pipeID, componentID string, cancel chan struct{}, in <-chan message, meter *meter) (<-chan message, <-chan error) {
	errc := make(chan error, 1)
	r.in = in
	r.out = make(chan message)
	go func() {
		defer close(r.out)
		defer close(errc)
		call(r.reset, pipeID, errc) // reset hook
		var err error
		var m message
		var ok bool
		for {
			// retrieve new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, pipeID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.applyTo(componentID)         // apply params
			m.buffer, err = r.fn(m.buffer) // process new buffer
			if err != nil {
				errc <- err
				return
			}

			meter = meter.sample(int64(m.buffer.Size())).message()

			m.feedback.applyTo(componentID) // apply feedback

			// send message further
			select {
			case r.out <- m:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}
		}
	}()
	return r.out, errc
}

// newSinkRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newSinkRunner(pipeID string, sampleRate, numChannels, bufferSize int, s Sink) (*sinkRunner, error) {
	fn, err := s.Sink(pipeID, sampleRate, numChannels, bufferSize)
	if err != nil {
		return nil, err
	}
	r := sinkRunner{
		fn:    fn,
		Sink:  s,
		hooks: bindHooks(s),
	}
	return &r, nil
}

// run the sink runner.
func (r *sinkRunner) run(pipeID, componentID string, cancel chan struct{}, in <-chan message, meter *meter) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		call(r.reset, pipeID, errc) // reset hook
		var m message
		var ok bool
		for {
			// receive new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, pipeID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.params.applyTo(componentID) // apply params
			err := r.fn(m.buffer)         // sink a buffer
			if err != nil {
				errc <- err
				return
			}

			meter = meter.sample(int64(m.buffer.Size())).message()

			m.feedback.applyTo(componentID) // apply feedback
		}
	}()

	return errc
}

// call optional function with pipeID argument. if error happens, it will be send to errc.
func call(fn hook, pipeID string, errc chan error) {
	if fn == nil {
		return
	}
	if err := fn(pipeID); err != nil {
		errc <- err
	}
	return
}

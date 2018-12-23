package pipe

import (
	"time"

	"github.com/dudk/phono"
)

// pumpRunner is pump's runner.
type pumpRunner struct {
	phono.Pump
	meter Meter
	fn    phono.PumpFunc
	out   chan message
	hooks
}

// processRunner represents processor's runner.
type processRunner struct {
	phono.Processor
	meter Meter
	fn    phono.ProcessFunc
	in    <-chan message
	out   chan message
	hooks
}

// sinkRunner represents sink's runner.
type sinkRunner struct {
	phono.Sink
	meter Meter
	fn    phono.SinkFunc
	in    <-chan message
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

// counters is a structure for metrics initialization.
var counters = struct {
	pump      []string
	processor []string
	sink      []string
}{
	pump:      []string{OutputCounter, StartCounter},
	processor: []string{OutputCounter, StartCounter},
	sink:      []string{OutputCounter, StartCounter},
}

var do struct{}

const (
	// OutputCounter is a key for output counter within metric.
	OutputCounter = "Output"
	// StartCounter is when runner started.
	StartCounter = "Start"
)

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
func newPumpRunner(sourceID string, p phono.Pump) (*pumpRunner, error) {
	fn, err := p.Pump(sourceID)
	if err != nil {
		return nil, err
	}
	r := pumpRunner{
		fn:    fn,
		Pump:  p,
		hooks: bindHooks(p),
	}
	return &r, nil
}

func (r *pumpRunner) setMetric(m Metric) {
	if m != nil {
		r.meter = m.Meter(r.ID(), counters.pump...)
	}
}

// run the Pump runner.
func (r *pumpRunner) run(cancel chan struct{}, sourceID string, provide chan struct{}, consume chan message) (<-chan message, <-chan error) {
	out := make(chan message)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		store(r.meter, StartCounter, time.Now())
		call(r.reset, sourceID, errc) // reset hook
		var err error
		var m message
		for {
			// request new message
			select {
			case provide <- do:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-consume:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.applyTo(r.ID())      // apply params
			m.Buffer, err = r.fn() // pump new buffer
			if err != nil {
				if err == phono.ErrEOP {
					call(r.flush, sourceID, errc) // flush hook
				} else {
					errc <- err
				}
				return
			}

			m.feedback.applyTo(r.ID()) // apply feedback

			// push message further
			select {
			case out <- m:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// newProcessRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newProcessRunner(sourceID string, p phono.Processor) (*processRunner, error) {
	fn, err := p.Process(sourceID)
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

func (r *processRunner) setMetric(m Metric) {
	if m != nil {
		r.meter = m.Meter(r.ID(), counters.processor...)
	}
}

// run the Processor runner.
func (r *processRunner) run(cancel chan struct{}, sourceID string, in <-chan message) (<-chan message, <-chan error) {
	errc := make(chan error, 1)
	r.in = in
	r.out = make(chan message)
	go func() {
		defer close(r.out)
		defer close(errc)
		store(r.meter, StartCounter, time.Now())
		call(r.reset, sourceID, errc) // reset hook
		var err error
		var m message
		var ok bool
		for {
			// retrieve new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, sourceID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.applyTo(r.Processor.ID())    // apply params
			m.Buffer, err = r.fn(m.Buffer) // process new buffer
			if err != nil {
				errc <- err
				return
			}
			m.feedback.applyTo(r.ID()) // apply feedback

			// send message further
			select {
			case r.out <- m:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}
		}
	}()
	return r.out, errc
}

// newSinkRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newSinkRunner(sourceID string, s phono.Sink) (*sinkRunner, error) {
	fn, err := s.Sink(sourceID)
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

func (r *sinkRunner) setMetric(m Metric) {
	if m != nil {
		r.meter = m.Meter(r.ID(), counters.sink...)
	}
}

// run the sink runner.
func (r *sinkRunner) run(cancel chan struct{}, sourceID string, in <-chan message) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		store(r.meter, StartCounter, time.Now())
		call(r.reset, sourceID, errc) // reset hook
		var m message
		var ok bool
		for {
			// receive new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, sourceID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.params.applyTo(r.Sink.ID()) // apply params
			err := r.fn(m.Buffer)         // sink a buffer
			if err != nil {
				errc <- err
				return
			}
			m.feedback.applyTo(r.ID()) // apply feedback
		}
	}()

	return errc
}

// call optional function with sourceID argument. if error happens, it will be send to errc.
func call(fn hook, sourceID string, errc chan error) {
	if fn == nil {
		return
	}
	if err := fn(sourceID); err != nil {
		errc <- err
	}
	return
}

func store(m Meter, c string, v interface{}) {
	if m != nil {
		m.Store(c, v)
	}
}

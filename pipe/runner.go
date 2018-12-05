package pipe

import (
	"github.com/dudk/phono"
)

// pumpRunner is pump's runner.
type pumpRunner struct {
	phono.Pump
	measurable
	fn  phono.PumpFunc
	out chan message
	hooks
}

// processRunner represents processor's runner.
type processRunner struct {
	phono.Processor
	measurable
	fn  phono.ProcessFunc
	in  <-chan message
	out chan message
	hooks
}

// sinkRunner represents sink's runner.
type sinkRunner struct {
	phono.Sink
	measurable
	fn phono.SinkFunc
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

// counters is a structure for metrics initialization.
var counters = struct {
	pump      []string
	processor []string
	sink      []string
}{
	pump:      []string{OutputCounter},
	processor: []string{OutputCounter},
	sink:      []string{OutputCounter},
}

var do struct{}

const (
	// OutputCounter is a key for output counter within metric.
	// It calculates regular total output per component.
	OutputCounter = "Output"
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

// build creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func (r *pumpRunner) build(sourceID string) (err error) {
	r.fn, err = r.Pump.Pump(sourceID)
	if err != nil {
		return err
	}
	return nil
}

// run the Pump runner.
func (r *pumpRunner) run(cancel chan struct{}, sourceID string, provide chan struct{}, consume chan message) (<-chan message, <-chan error) {
	out := make(chan message)
	errc := make(chan error, 1)
	r.measurable.Reset()
	go func() {
		defer close(out)
		defer close(errc)
		defer r.measurable.FinishMeasure()
		call(r.reset, sourceID, errc) // reset hook
		r.measurable.Latency()
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
			r.Counter(OutputCounter).Advance(m.Buffer)
			r.Latency()
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

// build creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func (r *processRunner) build(sourceID string) (err error) {
	r.fn, err = r.Processor.Process(sourceID)
	if err != nil {
		return err
	}
	return nil
}

// run the Processor runner.
func (r *processRunner) run(cancel chan struct{}, sourceID string, in <-chan message) (<-chan message, <-chan error) {
	errc := make(chan error, 1)
	r.in = in
	r.out = make(chan message)
	r.measurable.Reset()
	go func() {
		defer close(r.out)
		defer close(errc)
		defer r.measurable.FinishMeasure()
		call(r.reset, sourceID, errc) // reset hook
		r.measurable.Latency()
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
			r.Counter(OutputCounter).Advance(m.Buffer)
			r.Latency()
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

// build creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func (r *sinkRunner) build(sourceID string) (err error) {
	r.fn, err = r.Sink.Sink(sourceID)
	if err != nil {
		return err
	}
	return nil
}

// run the sink runner.
func (r *sinkRunner) run(cancel chan struct{}, sourceID string, in <-chan message) <-chan error {
	errc := make(chan error, 1)
	r.measurable.Reset()
	go func() {
		defer close(errc)
		defer r.measurable.FinishMeasure()
		call(r.reset, sourceID, errc) // reset hook
		r.measurable.Latency()
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
			r.Counter(OutputCounter).Advance(m.Buffer)
			r.Latency()
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

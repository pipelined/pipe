package runner

import (
	"io"

	"github.com/pipelined/signal"

	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/pipe/metric"
)

// Message is a main structure for pipe transport
type Message struct {
	SourceID string         // ID of pipe which spawned this message.
	Buffer   signal.Float64 // Buffer of message
	Params   state.Params   // params for pipe
}

// PumpFunc is closure of pipe.Pump that emits new messages.
type PumpFunc func(bufferSize int) ([][]float64, error)

// ProcessorFunc is closure of pipe.Processor that processes messages.
type ProcessorFunc func([][]float64) ([][]float64, error)

// SinkFunc is closure of pipe.Sink that sinks messages.
type SinkFunc func([][]float64) error

// Pump executes pipe.Pump components.
type Pump struct {
	Fn    PumpFunc
	Meter metric.ResetFunc
	Hooks
}

// Processor executes pipe.Processor components.
type Processor struct {
	Fn    ProcessorFunc
	Meter metric.ResetFunc
	Hooks
}

// Sink executes pipe.Sink components.
type Sink struct {
	Fn    SinkFunc
	Meter metric.ResetFunc
	Hooks
}

// Resetter is a component that must be resetted before new run.
// Reset hook is executed when Run happens.
type Resetter interface {
	Reset(string) error
}

// Interrupter defines component that has custom interruption logic.
// Interrupt hook is executed when Cancel happens.
type Interrupter interface {
	Interrupt(string) error
}

// Flusher defines component that must flushed in the end of execution.
// Flush hook is executed in the end of the run. It will be skipped if Reset hook has failed.
type Flusher interface {
	Flush(string) error
}

// hook represents optional functions for components lyfecycle.
type hook func(string) error

// Hooks is the set of components Hooks for runners.
type Hooks struct {
	flush     hook
	interrupt hook
	reset     hook
}

// BindHooks of component.
func BindHooks(v interface{}) Hooks {
	return Hooks{
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

// Run starts the Pump runner.
func (r *Pump) Run(bufferSize int, pipeID, componentID string, cancel <-chan struct{}, givec chan<- string, takec <-chan Message) (<-chan Message, <-chan error) {
	out := make(chan Message)
	errc := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errc)
		// reset hook
		if ok := call(r.reset, pipeID, errc); !ok {
			return
		}
		defer call(r.flush, pipeID, errc) // flush hook on return
		var err error
		var m Message
		for {
			// request new message
			select {
			case givec <- pipeID:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-takec:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.Params.ApplyTo(componentID)    // apply params
			m.Buffer, err = r.Fn(bufferSize) // pump new buffer
			// process buffer
			if m.Buffer != nil {
				meter(int64(m.Buffer.Size())) // capture metrics

				// push message further
				select {
				case out <- m:
				case <-cancel:
					call(r.interrupt, pipeID, errc) // interrupt hook
					return
				}
			}
			// handle error
			if err != nil {
				switch err {
				case io.EOF, io.ErrUnexpectedEOF:
					// run sucessfully completed
				default:
					errc <- err
				}
				return
			}
		}
	}()
	return out, errc
}

// Run starts the Processor runner.
func (r *Processor) Run(pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) (<-chan Message, <-chan error) {
	errc := make(chan error, 1)
	out := make(chan Message)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errc)
		// reset hook
		if ok := call(r.reset, pipeID, errc); !ok {
			return
		}
		defer call(r.flush, pipeID, errc) // flush hook on return
		var err error
		var m Message
		var ok bool
		for {
			// retrieve new message
			select {
			case m, ok = <-in:
				if !ok {
					return
				}
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.Params.ApplyTo(componentID)  // apply params
			m.Buffer, err = r.Fn(m.Buffer) // process new buffer
			if err != nil {
				errc <- err
				return
			}

			meter(int64(m.Buffer.Size())) // capture metrics

			// send message further
			select {
			case out <- m:
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// Run starts the sink runner.
func (r *Sink) Run(pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) <-chan error {
	errc := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(errc)
		// reset hook
		if ok := call(r.reset, pipeID, errc); !ok {
			return
		}
		defer call(r.flush, pipeID, errc) // flush hook on return
		var m Message
		var ok bool
		for {
			// receive new message
			select {
			case m, ok = <-in:
				if !ok {
					return
				}
			case <-cancel:
				call(r.interrupt, pipeID, errc) // interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params
			err := r.Fn(m.Buffer)         // sink a buffer
			if err != nil {
				errc <- err
				return
			}

			meter(int64(m.Buffer.Size())) // capture metrics
		}
	}()

	return errc
}

// call optional function with pipeID argument. If error happens, it will be send to errc.
// True is returned only if error happened.
func call(fn hook, pipeID string, errc chan error) bool {
	if fn == nil {
		return true
	}
	if err := fn(pipeID); err != nil {
		errc <- err
		return false
	}
	return true
}

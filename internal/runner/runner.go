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

type (
	// PumpFunc is closure of pipe.Pump that emits new messages.
	PumpFunc func(signal.Float64) error

	// ProcessFunc is closure of pipe.Processor that processes messages.
	ProcessFunc func(signal.Float64) error

	// SinkFunc is closure of pipe.Sink that sinks messages.
	SinkFunc func(signal.Float64) error

	// Pump executes pipe.Pump components.
	Pump struct {
		ID    string
		Fn    PumpFunc
		Meter metric.ResetFunc
		Hooks
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		ID    string
		Fn    ProcessFunc
		Meter metric.ResetFunc
		Hooks
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		ID    string
		Fn    SinkFunc
		Meter metric.ResetFunc
		Hooks
	}
)

type (
	// Hook represents optional functions for components lyfecycle.
	Hook func(string) error

	// Hooks is the set of components Hooks for runners.
	Hooks struct {
		Flush     Hook
		Interrupt Hook
		Reset     Hook
	}
)

var do struct{}

// Run starts the Pump runner.
func (r Pump) Run(bufferSize, numChannels int, pipeID, componentID string, cancel <-chan struct{}, givec chan<- string, takec <-chan Message) (<-chan Message, <-chan error) {
	out := make(chan Message)
	errc := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errc)
		// Reset hook
		if ok := call(r.Reset, pipeID, errc); !ok {
			return
		}
		defer call(r.Flush, pipeID, errc) // Flush hook on return
		var err error
		var m Message
		for {
			// request new message
			select {
			case givec <- pipeID:
			case <-cancel:
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-takec:
			case <-cancel:
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params

			// allocate new buffer
			m.Buffer = signal.Float64Buffer(numChannels, bufferSize, 0)

			err = r.Fn(m.Buffer)   // pump new buffer
			meter(m.Buffer.Size()) // capture metrics
			// handle error
			if err != nil {
				switch err {
				case io.EOF:
					// EOF is a good end.
				default:
					errc <- err
				}
				return
			}

			// push message further
			select {
			case out <- m:
			case <-cancel:
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// Run starts the Processor runner.
func (r Processor) Run(pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) (<-chan Message, <-chan error) {
	errc := make(chan error, 1)
	out := make(chan Message)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errc)
		// Reset hook
		if ok := call(r.Reset, pipeID, errc); !ok {
			return
		}
		defer call(r.Flush, pipeID, errc) // Flush hook on return
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
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params
			err = r.Fn(m.Buffer)          // process new buffer
			if err != nil {
				errc <- err
				return
			}

			meter(m.Buffer.Size()) // capture metrics

			// send message further
			select {
			case out <- m:
			case <-cancel:
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// Run starts the sink runner.
func (r Sink) Run(pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) <-chan error {
	errc := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(errc)
		// Reset hook
		if ok := call(r.Reset, pipeID, errc); !ok {
			return
		}
		defer call(r.Flush, pipeID, errc) // Flush hook on return
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
				call(r.Interrupt, pipeID, errc) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params
			err := r.Fn(m.Buffer)         // sink a buffer
			if err != nil {
				errc <- err
				return
			}

			meter(m.Buffer.Size()) // capture metrics
		}
	}()

	return errc
}

// call optional function with pipeID argument. If error happens, it will be send to errc.
// True is returned only if error happened.
func call(fn Hook, pipeID string, errc chan error) bool {
	if fn == nil {
		return true
	}
	if err := fn(pipeID); err != nil {
		errc <- err
		return false
	}
	return true
}

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
	ID    string
	Fn    PumpFunc
	Meter metric.ResetFunc
	Hooks
}

// Processor executes pipe.Processor components.
type Processor struct {
	ID    string
	Fn    ProcessorFunc
	Meter metric.ResetFunc
	Hooks
}

// Sink executes pipe.Sink components.
type Sink struct {
	ID    string
	Fn    SinkFunc
	Meter metric.ResetFunc
	Hooks
}

// Hook represents optional functions for components lyfecycle.
type Hook func(string) error

// Hooks is the set of components Hooks for runners.
type Hooks struct {
	Flush     Hook
	Interrupt Hook
	Reset     Hook
}

var do struct{}

// Run starts the Pump runner.
func (r Pump) Run(bufferSize int, pipeID, componentID string, cancel <-chan struct{}, givec chan<- string, takec <-chan Message) (<-chan Message, <-chan error) {
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

			m.Params.ApplyTo(componentID)    // apply params
			m.Buffer, err = r.Fn(bufferSize) // pump new buffer
			// process buffer
			if m.Buffer != nil {
				meter(int64(m.Buffer.Size())) // capture metrics

				// push message further
				select {
				case out <- m:
				case <-cancel:
					call(r.Interrupt, pipeID, errc) // Interrupt hook
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

			meter(int64(m.Buffer.Size())) // capture metrics
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

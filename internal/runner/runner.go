package runner

import (
	"io"
	"sync/atomic"

	"github.com/pipelined/signal"

	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/pipe/metric"
)

// Pool provides pooling of signal.Float64 buffers.
type Pool interface {
	Alloc() signal.Float64
	Free(signal.Float64)
}

// Message is a main structure for pipe transport
type Message struct {
	SinkRefs int32          // number of sinks referencing buffer in this message.
	PipeID   string         // ID of pipe which spawned this message.
	Buffer   signal.Float64 // Buffer of message.
	Params   state.Params   // params for pipe.
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
func (r Pump) Run(p Pool, pipeID, componentID string, cancel <-chan struct{}, give chan<- string, take <-chan Message) (<-chan Message, <-chan error) {
	out := make(chan Message, 1)
	errors := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errors)
		// Reset hook
		if ok := call(r.Reset, pipeID, errors); !ok {
			return
		}
		defer call(r.Flush, pipeID, errors) // Flush hook on return
		var err error
		var m Message
		for {
			// request new message
			select {
			case give <- pipeID:
			case <-cancel:
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-take:
			case <-cancel:
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params

			// POOL: Allocate buffer here.
			// allocate new buffer
			m.Buffer = p.Alloc()

			err = r.Fn(m.Buffer)   // pump new buffer
			meter(m.Buffer.Size()) // capture metrics
			// handle error
			if err != nil {
				switch err {
				case io.EOF:
					// EOF is a good end.
				default:
					errors <- err
				}
				return
			}

			// push message further
			select {
			case out <- m:
			case <-cancel:
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}
		}
	}()
	return out, errors
}

// Run starts the Processor runner.
func (r Processor) Run(pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) (<-chan Message, <-chan error) {
	errors := make(chan error, 1)
	out := make(chan Message, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errors)
		// Reset hook
		if ok := call(r.Reset, pipeID, errors); !ok {
			return
		}
		defer call(r.Flush, pipeID, errors) // Flush hook on return
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
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params
			err = r.Fn(m.Buffer)          // process new buffer
			if err != nil {
				errors <- err
				return
			}

			meter(m.Buffer.Size()) // capture metrics

			// send message further
			select {
			case out <- m:
			case <-cancel:
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}
		}
	}()
	return out, errors
}

// Run starts the sink runner.
func (r Sink) Run(p Pool, pipeID, componentID string, cancel <-chan struct{}, in <-chan Message) <-chan error {
	errors := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(errors)
		// Reset hook
		if ok := call(r.Reset, pipeID, errors); !ok {
			return
		}
		defer call(r.Flush, pipeID, errors) // Flush hook on return
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
				call(r.Interrupt, pipeID, errors) // Interrupt hook
				return
			}

			m.Params.ApplyTo(componentID) // apply params
			err := r.Fn(m.Buffer)         // sink a buffer
			if err != nil {
				errors <- err
				return
			}
			meter(m.Buffer.Size()) // capture metrics
			if atomic.AddInt32(&m.SinkRefs, -1) == 0 {
				p.Free(m.Buffer)
			}
		}
	}()

	return errors
}

// call optional function with pipeID argument. If error happens, it will be send to errors.
// True is returned only if error happened.
func call(fn Hook, pipeID string, errors chan error) bool {
	if fn == nil {
		return true
	}
	if err := fn(pipeID); err != nil {
		errors <- err
		return false
	}
	return true
}

// Broadcast passes messages to all sinks.
func Broadcast(p Pool, pipeID string, sinks []Sink, cancel <-chan struct{}, in <-chan Message) []<-chan error {
	//init errorss for sinks error channels
	errorss := make([]<-chan error, 0, len(sinks))
	//list of channels for broadcast
	broadcasts := make([]chan Message, len(sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan Message, 1)
	}

	//start broadcast
	for i, s := range sinks {
		errors := s.Run(p, pipeID, s.ID, cancel, broadcasts[i])
		errorss = append(errorss, errors)
	}

	go func() {
		//close broadcasts on return
		defer func() {
			for i := range broadcasts {
				close(broadcasts[i])
			}
		}()
		for msg := range in {
			for i := range broadcasts {
				m := Message{
					SinkRefs: int32(len(broadcasts)),
					PipeID:   pipeID,
					Buffer:   msg.Buffer,
					Params:   msg.Params.Detach(sinks[i].ID),
				}
				select {
				case broadcasts[i] <- m:
				case <-cancel:
					return
				}
			}
		}
	}()

	return errorss
}

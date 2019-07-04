package pipe

import (
	"github.com/rs/xid"

	"github.com/pipelined/signal"
)

// Pump is a source of samples. Pump method returns a new buffer with signal data.
// Implentetions should use next error conventions:
// 		- nil if a full buffer was read;
// 		- io.EOF if no data was read;
// 		- io.ErrUnexpectedEOF if not a full buffer was read.
// The latest case means that pump executed as expected, but not enough data was available.
// This incomplete buffer still will be sent further and pump will be finished gracefully.
// If no data was read or any other error was met, buffer should be nil.
type Pump interface {
	Pump(pipeID string, bufferSize int) (func() ([][]float64, error), int, int, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) ([][]float64, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error)
}

// Logger is a global interface for pipe loggers
type Logger interface {
	Debug(...interface{})
	Info(...interface{})
}

// newUID returns new unique id value.
func newUID() string {
	return xid.New().String()
}

// Message is a main structure for pipe transport
type message struct {
	buffer   signal.Float64 // Buffer of message
	params                  // params for pipe
	sourceID string         // ID of pipe which spawned this message.
	feedback params         //feedback are params applied after processing happened
}

// params represent a set of parameters mapped to ID of their receivers.
type params map[string][]func()

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	Pump
	Processors []Processor
	Sinks      []Sink
}

// Convert the event to a string.
func (e event) String() string {
	switch e {
	case run:
		return "run"
	case pause:
		return "pause"
	case resume:
		return "resume"
	case push:
		return "params"
	}
	return "unknown"
}

// Convert pipe to string. If name is included if has value.
// func (p *Pipe) String() string {
// 	return p.name
// }

// add appends a slice of params.
func (p params) add(componentID string, paramFuncs ...func()) params {
	var private []func()
	if _, ok := p[componentID]; !ok {
		private = make([]func(), 0, len(paramFuncs))
	}
	private = append(private, paramFuncs...)

	p[componentID] = private
	return p
}

// applyTo consumes params defined for consumer in this param set.
func (p params) applyTo(id string) {
	if p == nil {
		return
	}
	if params, ok := p[id]; ok {
		for _, param := range params {
			param()
		}
		delete(p, id)
	}
}

// merge two param sets into one.
func (p params) merge(source params) params {
	for newKey, newValues := range source {
		if _, ok := p[newKey]; ok {
			p[newKey] = append(p[newKey], newValues...)
		} else {
			p[newKey] = newValues
		}
	}
	return p
}

func (p params) detach(id string) params {
	if p == nil {
		return nil
	}
	if v, ok := p[id]; ok {
		d := params(make(map[string][]func()))
		d[id] = v
		return d
	}
	return nil
}

type silentLogger struct{}

func (silentLogger) Debug(args ...interface{}) {}

func (silentLogger) Info(args ...interface{}) {}

var defaultLogger silentLogger

package pipe

import (
	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/signal"
	"github.com/rs/xid"
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
	Pump(pipeID string) (func(bufferSize int) ([][]float64, error), int, int, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(pipeID string, sampleRate, numChannels int) (func([][]float64) ([][]float64, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(pipeID string, sampleRate, numChannels int) (func([][]float64) error, error)
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

// message is a main structure for pipe transport
type message struct {
	sourceID string         // ID of pipe which spawned this message.
	buffer   signal.Float64 // Buffer of message
	params   state.Params   // params for pipe
	// Feedback     state.Params   //feedback are params applied after processing happened
}

// Convert pipe to string. If name is included if has value.
// func (p *Pipe) String() string {
// 	return p.name
// }

// Wait for state transition or first error to occur.
func Wait(d chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

type silentLogger struct{}

func (silentLogger) Debug(args ...interface{}) {}

func (silentLogger) Info(args ...interface{}) {}

var defaultLogger silentLogger

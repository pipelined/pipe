package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"
	"pipelined.dev/signal/pool"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/internal/state"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutator"
)

// pipeline components
type (
	PumpFunc func() (Pump, signal.SampleRate, int, error)
	// Pump is a source of samples. Pump method returns a new buffer with signal data.
	// If no data is available, io.EOF should be returned. If pump cannot provide data
	// to fulfill buffer, it can trim the size of the buffer to align it with actual data.
	// Buffer size can only be decreased.
	Pump struct {
		Pump func(out signal.Float64) error
		Flush
	}

	ProcessorFunc func(signal.SampleRate, int) (Processor, signal.SampleRate, int, error)
	// Processor defines interface for pipe processors.
	// Processor should return output in the same signal buffer as input.
	// It is encouraged to implement in-place processing algorithms.
	// Buffer size could be changed during execution, but only decrease allowed.
	// Number of channels cannot be changed.
	Processor struct {
		Process func(in, out signal.Float64) error
		Flush
	}

	SinkFunc func(signal.SampleRate, int) (Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. L can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		Sink func(in signal.Float64) error
		Flush
	}

	Flush func(context.Context) error
)

// L (Line) is a sequence of DSP components.
type L struct {
	numChannels int
	mutators    chan mutator.Mutators
	pump        runner.Pump
	processors  []runner.Processor
	sinks       []runner.Sink
}

// Routing defines sequence of components closures.
// It has a single pump, zero or many processors, executed
// sequentially and one or many sinks executed in parallel.
type Routing struct {
	Pump       PumpFunc
	Processors []ProcessorFunc
	Sinks      []SinkFunc
}

// Pipe controls the execution of multiple chained lines. Lines might be chained
// through components, mixer for example.  If lines are not chained, they must be
// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
type Pipe struct {
	handle *state.Handle
	lines  map[chan mutator.Mutators]*L // map chain id to chain
}

func Line(r Routing) (*L, error) {
	// try to bind all lines
	line, err := r.bind()
	if err != nil {
		return nil, fmt.Errorf("error binding %w", err)
	}
	return line, nil
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ls ...*L) *Pipe {
	lines := make(map[chan mutator.Mutators]*L)
	for _, l := range ls {
		l.mutators = make(chan mutator.Mutators)
		lines[l.mutators] = l
	}
	handle := state.NewHandle(startFunc(lines))
	go state.Loop(handle)
	return &Pipe{
		lines:  lines,
		handle: handle,
	}
}

func (r Routing) bind() (*L, error) {
	var (
		line        L
		pump        Pump
		processor   Processor
		sink        Sink
		numChannels int
		sampleRate  signal.SampleRate
		err         error
	)
	pump, sampleRate, numChannels, err = r.Pump()
	if err != nil {
		return nil, fmt.Errorf("pump: %w", err)
	}
	line.pump = runner.Pump{
		SampleRate:  sampleRate,
		NumChannels: numChannels,
		Fn:          pump.Pump,
		Flush:       pump.Flush,
		Meter:       metric.Meter(pump, signal.SampleRate(sampleRate)),
	}

	// bind processors
	for _, fn := range r.Processors {
		processor, sampleRate, numChannels, err = fn(sampleRate, numChannels)
		if err != nil {
			return nil, fmt.Errorf("processor: %w", err)
		}
		line.processors = append(line.processors,
			runner.Processor{
				SampleRate:  sampleRate,
				NumChannels: numChannels,
				Fn:          processor.Process,
				Flush:       processor.Flush,
				Meter:       metric.Meter(processor, signal.SampleRate(sampleRate)),
			},
		)
	}

	// bind sinks
	for _, sinkFn := range r.Sinks {
		sink, err = sinkFn(sampleRate, numChannels)
		if err != nil {
			return nil, fmt.Errorf("sink: %w", err)
		}
		line.sinks = append(line.sinks,
			runner.Sink{
				SampleRate:  sampleRate,
				NumChannels: numChannels,
				Fn:          sink.Sink,
				Flush:       sink.Flush,
				Meter:       metric.Meter(sink, signal.SampleRate(sampleRate)),
			},
		)
	}
	return &line, nil
}

// start starts the execution of pipe.
func startFunc(lines map[chan mutator.Mutators]*L) state.StartFunc {
	return func(ctx context.Context, bufferSize int, give chan<- chan mutator.Mutators) ([]<-chan error, error) {
		// start all runners
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for mutators, l := range lines {
			p := pool.New(l.numChannels, bufferSize)
			// start pump
			out, errs := l.pump.Run(ctx, p, give, mutators)
			errcList = append(errcList, errs)

			// start chained processesing
			for _, proc := range l.processors {
				out, errs = proc.Run(ctx, out)
				errcList = append(errcList, errs)
			}

			sinkErrcList := runner.Broadcast(ctx, p, l.sinks, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList, nil
	}
}

// Run sends a run event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback channel is closed when Ready state is reached or context is cancelled.
func (p *Pipe) Run(ctx context.Context, bufferSize int) chan error {
	return p.handle.Run(ctx, bufferSize)
}

// Pause sends a pause event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Paused state is reached.
func (p *Pipe) Pause() chan error {
	return p.handle.Pause()
}

// Resume sends a resume event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Ready state is reached.
func (p *Pipe) Resume() chan error {
	return p.handle.Resume()
}

// Close must be called to clean up handle's resources.
// Feedback is closed when line is done.
func (p *Pipe) Close() chan error {
	return p.handle.Interrupt()
}

// Push new mutators into pipe.
// Calling this method after pipe is closed causes a panic.
// TODO: figure how to expose receivers.
func (p *Pipe) push(r *mutator.Receiver, paramFuncs ...func()) {
	p.handle.Push(mutator.Mutators{r: paramFuncs})
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorFunc) []ProcessorFunc {
	return processors
}

// Sinks is a helper function to use in line constructors.
func Sinks(sinks ...SinkFunc) []SinkFunc {
	return sinks
}

// Wait for state transition or first error to occur.
func Wait(d <-chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

func flatenErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return fmt.Errorf("%v", errs[0])
	}

	var b []byte
	for _, err := range errs {
		b = append(b, err.Error()...)
		b = append(b, '\n')
	}
	return fmt.Errorf("%s\nin total:%d", b, len(errs))
}

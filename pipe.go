package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/pool"
	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutator"
)

// pipeline components
type (
	PumpFunc func(bufferSize int) (Pump, signal.SampleRate, int, error)
	// Pump is a source of samples. Pump method returns a new buffer with signal data.
	// If no data is available, io.EOF should be returned. If pump cannot provide data
	// to fulfill buffer, it can trim the size of the buffer to align it with actual data.
	// Buffer size can only be decreased.
	Pump struct {
		Pump func(out signal.Float64) error
		Flush
	}

	ProcessorFunc func(buffersize int, sr signal.SampleRate, numChannels int) (Processor, signal.SampleRate, int, error)
	// Processor defines interface for pipe processors.
	// Processor should return output in the same signal buffer as input.
	// It is encouraged to implement in-place processing algorithms.
	// Buffer size could be changed during execution, but only decrease allowed.
	// Number of channels cannot be changed.
	Processor struct {
		Process func(in, out signal.Float64) error
		Flush
	}

	SinkFunc func(buffersize int, sr signal.SampleRate, numChannels int) (Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. Route can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		Sink func(in signal.Float64) error
		Flush
	}

	Flush func(context.Context) error
)

type (
	// Route is a sequence of DSP components.
	Route struct {
		numChannels int
		mutators    chan mutator.Mutators
		pump        runner.Pump
		processors  []runner.Processor
		sink        runner.Sink
	}

	// Line defines sequence of components closures.
	// It has a single pump, zero or many processors, executed
	// sequentially and one or many sinks executed in parallel.
	Line struct {
		Pump       PumpFunc
		Processors []ProcessorFunc
		Sink       SinkFunc
	}

	// Pipe controls the execution of multiple chained lines. Lines might be chained
	// through components, mixer for example.  If lines are not chained, they must be
	// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
	Pipe struct {
		ctx      context.Context
		cancelFn context.CancelFunc
		merger   *merger
		routes   map[chan mutator.Mutators]Route
		mutators map[chan mutator.Mutators]mutator.Mutators
		pull     chan chan mutator.Mutators
		push     chan []Mutation
		errors   chan error
	}
)

type Handle struct {
	puller   chan mutator.Mutators
	receiver *mutator.Receiver
}

type Mutation struct {
	Handle
	Mutators []mutator.Mutator
}

func (r Route) Pump() Handle {
	return Handle{
		puller:   r.mutators,
		receiver: r.pump.Receiver,
	}
}

func (r Route) Processors() []Handle {
	if len(r.processors) == 0 {
		return nil
	}
	handles := make([]Handle, 0, len(r.processors))
	for _, p := range r.processors {
		handles = append(handles,
			Handle{
				puller:   r.mutators,
				receiver: p.Receiver,
			})
	}
	return handles
}

func (r Route) Sink() Handle {
	return Handle{
		puller:   r.mutators,
		receiver: r.sink.Receiver,
	}
}

// Route line components. All closures are executed and wrapped into runners.
func (l Line) Route(bufferSize int) (Route, error) {
	pump, err := l.Pump.runner(bufferSize)
	if err != nil {
		return Route{}, fmt.Errorf("error routing %w", err)
	}
	input := pump.Output

	var processors []runner.Processor
	for _, fn := range l.Processors {
		processor, err := fn.runner(bufferSize, input)
		if err != nil {
			return Route{}, fmt.Errorf("error routing %w", err)
		}
		processors, input = append(processors, processor), processor.Output
	}

	sink, err := l.Sink.runner(bufferSize, input)
	if err != nil {
		return Route{}, fmt.Errorf("error routing sink: %w", err)
	}

	return Route{
		mutators:   make(chan mutator.Mutators),
		pump:       pump,
		processors: processors,
		sink:       sink,
	}, nil
}

func (fn PumpFunc) runner(bufferSize int) (runner.Pump, error) {
	pump, sampleRate, numChannels, err := fn(bufferSize)
	if err != nil {
		return runner.Pump{}, fmt.Errorf("pump: %w", err)
	}
	return runner.Pump{
		Receiver: &mutator.Receiver{},
		Output: runner.Bus{
			SampleRate:  sampleRate,
			NumChannels: numChannels,
			Pool:        pool.Get(bufferSize, numChannels),
		},
		Fn:    pump.Pump,
		Flush: runner.Flush(pump.Flush),
		Meter: metric.Meter(pump, sampleRate),
	}, nil
}

func (fn ProcessorFunc) runner(bufferSize int, input runner.Bus) (runner.Processor, error) {
	processor, sampleRate, numChannels, err := fn(bufferSize, input.SampleRate, input.NumChannels)
	if err != nil {
		return runner.Processor{}, fmt.Errorf("processor: %w", err)
	}
	return runner.Processor{
		Receiver: &mutator.Receiver{},
		Input:    input,
		Output: runner.Bus{
			SampleRate:  sampleRate,
			NumChannels: numChannels,
			Pool:        pool.Get(bufferSize, numChannels),
		},
		Fn:    processor.Process,
		Flush: runner.Flush(processor.Flush),
		Meter: metric.Meter(processor, sampleRate),
	}, nil
}

func (fn SinkFunc) runner(bufferSize int, input runner.Bus) (runner.Sink, error) {
	sink, err := fn(bufferSize, input.SampleRate, input.NumChannels)
	if err != nil {
		return runner.Sink{}, fmt.Errorf("sink: %w", err)
	}
	return runner.Sink{
		Receiver: &mutator.Receiver{},
		Input:    input,
		Fn:       sink.Sink,
		Flush:    runner.Flush(sink.Flush),
		Meter:    metric.Meter(sink, input.SampleRate),
	}, nil
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
// TODO: consider options
func New(ctx context.Context, options ...Option) Pipe {
	ctx, cancelFn := context.WithCancel(ctx)
	p := Pipe{
		merger: &merger{
			errors: make(chan error, 1),
		},
		ctx:      ctx,
		cancelFn: cancelFn,
		mutators: make(map[chan mutator.Mutators]mutator.Mutators),
		routes:   make(map[chan mutator.Mutators]Route),
		pull:     make(chan chan mutator.Mutators),
		push:     make(chan []Mutation),
		errors:   make(chan error, 1),
	}
	for _, option := range options {
		option(&p)
	}
	if len(p.routes) == 0 {
		panic("pipe without routes")
	}
	// options are before this step
	p.merger.merge(start(p.ctx, p.pull, p.routes)...)

	go func() {
		defer close(p.errors)
		for {
			select {
			case mutators := <-p.push:
				for _, m := range mutators {
					p.mutators[m.Handle.puller] = p.mutators[m.Handle.puller].Add(m.Handle.receiver, m.Mutators...)
				}
			case puller := <-p.pull:
				mutators := p.mutators[puller]
				delete(p.mutators, puller)
				puller <- mutators
				continue
			case err, ok := <-p.merger.errors:
				// merger has buffer of one error,
				// if more errors happen, they will be ignored.
				if ok {
					p.cancelFn()
					// wait until all groutines stop.
					for {
						if _, ok = <-p.merger.errors; !ok {
							break
						}
					}
					p.errors <- fmt.Errorf("pipe error: %w", err)
				}
				return
			}
		}
	}()

	return p
}

// start starts the execution of pipe.
func start(ctx context.Context, pull chan<- chan mutator.Mutators, routes map[chan mutator.Mutators]Route) []<-chan error {
	// start all runners
	// error channel for each component
	errcList := make([]<-chan error, 0)
	for mutators, r := range routes {
		// start pump
		out, errs := r.pump.Run(ctx, pull, mutators)
		errcList = append(errcList, errs)

		// start chained processesing
		for _, proc := range r.processors {
			out, errs = proc.Run(ctx, out)
			errcList = append(errcList, errs)
		}

		errs = r.sink.Run(ctx, out)
		errcList = append(errcList, errs)
	}
	return errcList
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (p Pipe) Push(mutations ...Mutation) {
	p.push <- mutations
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorFunc) []ProcessorFunc {
	return processors
}

// Wait for state transition or first error to occur.
func (p Pipe) Wait() error {
	for err := range p.errors {
		if err != nil {
			return err
		}
	}
	return nil
}

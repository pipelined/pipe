package pipe

import (
	"context"
	"crypto/rand"
	"fmt"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/pool"
	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutate"
)

type (
	Bus struct {
		BufferSize int
		signal.SampleRate
		NumChannels int
	}
	// PumpMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	PumpMaker func(bufferSize int) (ID, Pump, Bus, error)
	// Pump is a source of samples. Pump method accepts a new buffer and
	// fills it with signal data. If no data is available, io.EOF should
	// be returned. If pump cannot provide data to fulfill buffer, it can
	// trim the size of the buffer to align it with actual data.
	// Buffer size can only be decreased.
	Pump struct {
		Pump func(out signal.Float64) error
		Flush
	}

	// ProcessorMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	ProcessorMaker func(Bus) (ID, Processor, Bus, error)
	// Processor defines interface for pipe processors. It receives two
	// buffers for input and output signal data. Buffer size could be
	// changed during execution, but only decrease allowed. Number of
	// channels cannot be changed.
	Processor struct {
		Process func(in, out signal.Float64) error
		Flush
	}

	// SinkMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	SinkMaker func(Bus) (ID, Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. Route can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		Sink func(in signal.Float64) error
		Flush
	}

	// Flush provides a hook to flush all buffers for the component.
	Flush func(context.Context) error
)

type (
	// Route is a sequence of DSP components.
	Route struct {
		numChannels int
		mutators    chan mutate.Mutators
		receivers   map[ID]struct{}
		pump        runner.Pump
		processors  []runner.Processor
		sink        runner.Sink
	}

	// Line defines sequence of components closures.
	// It has a single pump, zero or many processors, executed
	// sequentially and one or many sinks executed in parallel.
	Line struct {
		Pump       PumpMaker
		Processors []ProcessorMaker
		Sink       SinkMaker
	}

	// Pipe controls the execution of multiple chained lines. Lines might be chained
	// through components, mixer for example.  If lines are not chained, they must be
	// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
	Pipe struct {
		id       ID
		ctx      context.Context
		cancelFn context.CancelFunc
		merger   *merger
		routes   []Route
		mutators map[chan mutate.Mutators]mutate.Mutators
		pull     chan chan mutate.Mutators
		push     chan []Mutation
		errors   chan error
	}
)

type (
	// ID of the DSP line.
	ID [16]byte

	// Mutation is a set of mutators attached to a specific component.
	Mutation struct {
		ID
		Mutators []mutate.Mutator
		puller   chan mutate.Mutators
	}
)

func (c ID) Mutate(ms ...mutate.Mutator) Mutation {
	return Mutation{
		ID:       c,
		Mutators: ms,
	}
}

func (r Route) Pump() ID {
	return r.pump.ID
}

func (r Route) Processors() []ID {
	if len(r.processors) == 0 {
		return nil
	}
	components := make([]ID, 0, len(r.processors))
	for _, p := range r.processors {
		components = append(components, p.ID)
	}
	return components
}

func (r Route) Sink() ID {
	return r.sink.ID
}

func NewID() ID {
	var id [16]byte
	rand.Read(id[:])
	return id
}

// Route line components. All closures are executed and wrapped into runners.
func (l Line) Route(bufferSize int) (Route, error) {
	receivers := make(map[ID]struct{})
	pump, input, err := l.Pump.runner(bufferSize)
	if err != nil {
		return Route{}, fmt.Errorf("error routing %w", err)
	}
	receivers[pump.ID] = struct{}{}

	var (
		processors []runner.Processor
		processor  runner.Processor
	)
	for _, fn := range l.Processors {
		processor, input, err = fn.runner(bufferSize, input)
		if err != nil {
			return Route{}, fmt.Errorf("error routing %w", err)
		}
		processors = append(processors, processor)
		receivers[processor.ID] = struct{}{}
	}

	sink, err := l.Sink.runner(bufferSize, input)
	if err != nil {
		return Route{}, fmt.Errorf("error routing sink: %w", err)
	}
	receivers[sink.ID] = struct{}{}

	return Route{
		mutators:   make(chan mutate.Mutators),
		receivers:  receivers,
		pump:       pump,
		processors: processors,
		sink:       sink,
	}, nil
}

func (fn PumpMaker) runner(bufferSize int) (runner.Pump, Bus, error) {
	id, pump, bus, err := fn(bufferSize)
	if err != nil {
		return runner.Pump{}, Bus{}, fmt.Errorf("pump: %w", err)
	}
	return runner.Pump{
		ID:     id,
		Output: pool.Get(bufferSize, bus.NumChannels),
		Fn:     pump.Pump,
		Flush:  runner.Flush(pump.Flush),
		Meter:  metric.Meter(pump, bus.SampleRate),
	}, bus, nil
}

func (fn ProcessorMaker) runner(bufferSize int, input Bus) (runner.Processor, Bus, error) {
	id, processor, output, err := fn(input)
	if err != nil {
		return runner.Processor{}, Bus{}, fmt.Errorf("processor: %w", err)
	}
	return runner.Processor{
		ID:     id,
		Input:  pool.Get(bufferSize, input.NumChannels),
		Output: pool.Get(bufferSize, output.NumChannels),
		Fn:     processor.Process,
		Flush:  runner.Flush(processor.Flush),
		Meter:  metric.Meter(processor, output.SampleRate),
	}, output, nil
}

func (fn SinkMaker) runner(bufferSize int, input Bus) (runner.Sink, error) {
	id, sink, err := fn(input)
	if err != nil {
		return runner.Sink{}, fmt.Errorf("sink: %w", err)
	}
	return runner.Sink{
		ID:    id,
		Input: pool.Get(bufferSize, input.NumChannels),
		Fn:    sink.Sink,
		Flush: runner.Flush(sink.Flush),
		Meter: metric.Meter(sink, input.SampleRate),
	}, nil
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ctx context.Context, options ...Option) Pipe {
	ctx, cancelFn := context.WithCancel(ctx)
	p := Pipe{
		id: NewID(),
		merger: &merger{
			errors: make(chan error, 1),
		},
		ctx:      ctx,
		cancelFn: cancelFn,
		mutators: make(map[chan mutate.Mutators]mutate.Mutators),
		routes:   make([]Route, 0),
		pull:     make(chan chan mutate.Mutators),
		push:     make(chan []Mutation, 1),
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
	go p.merger.wait()
	go func() {
		defer close(p.errors)
		for {
			select {
			case mutations := <-p.push:
				for _, m := range mutations {
					// mutate pipe itself
					if m.ID == p.id {
						for _, fn := range m.Mutators {
							if err := fn(); err != nil {
								p.interrupt(err)
							}
						}
					} else {
						p.mutators[m.puller] = p.mutators[m.puller].Add(m.ID, m.Mutators...)
					}
				}
			case puller := <-p.pull:
				mutators := p.mutators[puller]
				p.mutators[puller] = nil
				puller <- mutators
				continue
			case err, ok := <-p.merger.errors:
				// merger has buffer of one error,
				// if more errors happen, they will be ignored.
				if ok {
					p.interrupt(err)
				}
				return
			}
		}
	}()
	return p
}

func (p Pipe) interrupt(err error) {
	p.cancelFn()
	// wait until all groutines stop.
	for {
		// only the first error is propagated.
		if _, ok := <-p.merger.errors; !ok {
			break
		}
	}
	p.errors <- fmt.Errorf("pipe error: %w", err)
}

// start starts the execution of pipe.
func start(ctx context.Context, pull chan<- chan mutate.Mutators, routes []Route) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, 2*len(routes))
	for _, r := range routes {
		errChans = append(errChans, r.start(ctx, pull)...)
	}
	return errChans
}

func (r Route) start(ctx context.Context, pull chan<- chan mutate.Mutators) []<-chan error {
	errChans := make([]<-chan error, 0, 2+len(r.processors))
	// start pump
	out, errs := r.pump.Run(ctx, pull, r.mutators)
	errChans = append(errChans, errs)

	// start chained processesing
	for _, proc := range r.processors {
		out, errs = proc.Run(ctx, out)
		errChans = append(errChans, errs)
	}

	errs = r.sink.Run(ctx, out)
	errChans = append(errChans, errs)
	return errChans
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (p Pipe) Push(mutations ...Mutation) {
	p.push <- p.filterMutations(mutations)
}

// Filter mutations that belong to the current pipe.
func (p Pipe) filterMutations(mutations []Mutation) []Mutation {
	present := 0
	for i := range mutations {
		if mutations[i].ID == p.id {
			mutations[present] = mutations[i]
			present++
		} else if puller := p.getPuller(mutations[i].ID); puller != nil {
			mutations[present] = mutations[i]
			mutations[present].puller = puller
			present++
		}
	}
	return mutations[:present]
}

func (p Pipe) getPuller(id ID) chan mutate.Mutators {
	for _, route := range p.routes {
		if _, ok := route.receivers[id]; ok {
			return route.mutators
		}
	}
	return nil
}

func (p Pipe) AddRoute(r Route) Mutation {
	return Mutation{
		ID: p.id,
		Mutators: []mutate.Mutator{
			func() error {
				p.merger.merge(r.start(p.ctx, p.pull)...)
				return nil
			},
		},
	}
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorMaker) []ProcessorMaker {
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

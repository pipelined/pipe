package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/internal/state"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/signal/pool"
)

// Pipe controls the execution of multiple chained lines. Lines might be chained
// through components, mixer for example.  If lines are not chained, they must be
// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
type Pipe struct {
	h      *state.Handle
	lines  map[*Line]string             // map pipe to chain id
	chains map[chan runner.Params]chain // map chain id to chain
}

// chain is a runtime chain of the pipeline.
type chain struct {
	params      chan runner.Params
	sampleRate  signal.SampleRate
	numChannels int
	pump        runner.Pump
	processors  []runner.Processor
	sinks       []runner.Sink
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ls ...*Line) (*Pipe, error) {
	lines := make(map[*Line]string)
	chains := make(map[chan runner.Params]chain)
	// chainByComponent := make(map[string]string)
	for _, p := range ls {
		// bind all lines
		c, err := bindLine(p)
		if err != nil {
			return nil, fmt.Errorf("error binding line: %w", err)
		}
		// map chains
		chains[c.params] = c
	}

	p := &Pipe{
		lines:  lines,
		chains: chains,
	}
	p.h = state.NewHandle(start(p))
	go state.Loop(p.h)
	return p, nil
}

func bindLine(p *Line) (chain, error) {
	// bind pump
	pump, sampleRate, numChannels, err := p.Pump()
	if err != nil {
		return chain{}, fmt.Errorf("pump: %w", err)
	}
	pumpRunner := runner.Pump{
		Fn:    runner.PumpFunc(pump.Pump),
		Meter: metric.Meter(p.Pump, signal.SampleRate(sampleRate)),
		Hooks: BindHooks(p.Pump),
	}

	// bind processors
	processorRunners := make([]runner.Processor, 0, len(p.Processors))
	for _, processorFn := range p.Processors {
		processor, sampleRate, _, err := processorFn(sampleRate, numChannels)
		if err != nil {
			return chain{}, fmt.Errorf("processor: %w", err)
		}
		processorRunner := runner.Processor{
			Fn:    runner.ProcessFunc(processor.Process),
			Meter: metric.Meter(processor, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(processor),
		}
		processorRunners = append(processorRunners, processorRunner)
	}

	// bind sinks
	sinkRunners := make([]runner.Sink, 0, len(p.Sinks))
	for _, sinkFn := range p.Sinks {
		sink, err := sinkFn(sampleRate, numChannels)
		if err != nil {
			return chain{}, fmt.Errorf("sink: %w", err)
		}
		sinkRunner := runner.Sink{
			Fn:    runner.SinkFunc(sink.Sink),
			Meter: metric.Meter(sink, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(sink),
		}
		sinkRunners = append(sinkRunners, sinkRunner)
	}
	return chain{
		sampleRate:  sampleRate,
		numChannels: numChannels,
		pump:        pumpRunner,
		processors:  processorRunners,
		sinks:       sinkRunners,
		params:      make(chan runner.Params),
	}, nil
}

// start starts the execution of pipe.
func start(p *Pipe) state.StartFunc {
	return func(bufferSize int, cancel <-chan struct{}, give chan<- chan runner.Params) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range p.chains {
			p := pool.New(c.numChannels, bufferSize)
			// start pump
			out, errs := c.pump.Run(p, cancel, give, c.params)
			errcList = append(errcList, errs)

			// start chained processesing
			for _, proc := range c.processors {
				out, errs = proc.Run(cancel, out)
				errcList = append(errcList, errs)
			}

			sinkErrcList := runner.Broadcast(p, c.sinks, cancel, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList
	}
}

// Run sends a run event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback channel is closed when Ready state is reached or context is cancelled.
func (p *Pipe) Run(ctx context.Context, bufferSize int) chan error {
	return p.h.Run(ctx, bufferSize)
}

// Pause sends a pause event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Paused state is reached.
func (p *Pipe) Pause() chan error {
	return p.h.Pause()
}

// Resume sends a resume event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Ready state is reached.
func (p *Pipe) Resume() chan error {
	return p.h.Resume()
}

// Close must be called to clean up handle's resources.
// Feedback is closed when line is done.
func (p *Pipe) Close() chan error {
	return p.h.Interrupt()
}

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Push(id string, paramFuncs ...func()) {
	p.h.Push(runner.Params{id: paramFuncs})
}

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
	h     *state.Handle
	lines map[chan runner.Params]*Line // map chain id to chain
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ls ...*Line) (*Pipe, error) {
	lines := make(map[chan runner.Params]*Line)
	for _, l := range ls {
		// bind all lines
		err := bindLine(l)
		if err != nil {
			return nil, fmt.Errorf("error binding line: %w", err)
		}
		// map chains
		lines[l.params] = l
	}

	p := &Pipe{
		lines: lines,
	}
	p.h = state.NewHandle(start(p))
	go state.Loop(p.h)
	return p, nil
}

func bindLine(l *Line) error {
	// bind pump
	pump, sampleRate, numChannels, err := l.Pump()
	if err != nil {
		return fmt.Errorf("error binding pump: %w", err)
	}
	pumpRunner := runner.Pump{
		Fn:    runner.PumpFunc(pump.Pump),
		Meter: metric.Meter(l.Pump, signal.SampleRate(sampleRate)),
		Hooks: BindHooks(l.Pump),
	}

	// bind processors
	processorRunners := make([]runner.Processor, 0, len(l.Processors))
	for _, processorFn := range l.Processors {
		processor, _, _, err := processorFn(sampleRate, numChannels)
		if err != nil {
			return fmt.Errorf("error binding processor: %w", err)
		}
		processorRunner := runner.Processor{
			Fn:    runner.ProcessFunc(processor.Process),
			Meter: metric.Meter(processor, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(processor),
		}
		processorRunners = append(processorRunners, processorRunner)
	}

	// bind sinks
	sinkRunners := make([]runner.Sink, 0, len(l.Sinks))
	for _, sinkFn := range l.Sinks {
		sink, err := sinkFn(sampleRate, numChannels)
		if err != nil {
			return fmt.Errorf("sink: %w", err)
		}
		sinkRunner := runner.Sink{
			Fn:    runner.SinkFunc(sink.Sink),
			Meter: metric.Meter(sink, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(sink),
		}
		sinkRunners = append(sinkRunners, sinkRunner)
	}
	l.numChannels = numChannels
	l.pump = pumpRunner
	l.processors = processorRunners
	l.sinks = sinkRunners
	l.params = make(chan runner.Params)
	return nil
}

// start starts the execution of pipe.
func start(p *Pipe) state.StartFunc {
	return func(bufferSize int, cancel <-chan struct{}, give chan<- chan runner.Params) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range p.lines {
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

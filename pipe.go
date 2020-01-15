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

// pipeline components
type (
	PumpFunc func() (Pump, signal.SampleRate, int, error)
	// Pump is a source of samples. Pump method returns a new buffer with signal data.
	// If no data is available, io.EOF should be returned. If pump cannot provide data
	// to fulfill buffer, it can trim the size of the buffer to align it with actual data.
	// Buffer size can only be decreased.
	Pump struct {
		Pump func(out signal.Float64) error
		Hooks
	}

	ProcessorFunc func(signal.SampleRate, int) (Processor, signal.SampleRate, int, error)
	// Processor defines interface for pipe processors.
	// Processor should return output in the same signal buffer as input.
	// It is encouraged to implement in-place processing algorithms.
	// Buffer size could be changed during execution, but only decrease allowed.
	// Number of channels cannot be changed.
	Processor struct {
		Process func(in, out signal.Float64) error
		Hooks
	}

	SinkFunc func(signal.SampleRate, int) (Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. Line can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		Sink func(in signal.Float64) error
		Hooks
	}

	Hook func() error

	Hooks struct {
		Flush     Hook
		Interrupt Hook
		Reset     Hook
	}
)

// optional interfaces
type (
	// Resetter is a component that must be resetted before new run.
	// Reset hook is executed when Run happens.
	Resetter interface {
		Reset() error
	}

	// Interrupter is a component that has custom interruption logic.
	// Interrupt hook is triggered when pipe run is cancelled.
	Interrupter interface {
		Interrupt() error
	}

	// Flusher is a component that must be flushed in the end of execution.
	// Flush hook is executed in the end of the run. It will be skipped if Reset hook has failed.
	Flusher interface {
		Flush() error
	}
)

// Line is a sound processing sequence of components.
// It has a single pump, zero or many processors executed sequentially
// and one or many sinks executed in parallel.
type Line struct {
	Pump       PumpFunc
	Processors []ProcessorFunc
	Sinks      []SinkFunc

	numChannels int
	params      chan state.Params
	pump        runner.Pump
	processors  []runner.Processor
	sinks       []runner.Sink
}

// Pipe controls the execution of multiple chained lines. Lines might be chained
// through components, mixer for example.  If lines are not chained, they must be
// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
type Pipe struct {
	handle *state.Handle
	lines  map[chan state.Params]*Line // map chain id to chain
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ls ...*Line) *Pipe {
	lines := make(map[chan state.Params]*Line)
	for _, l := range ls {
		l.params = make(chan state.Params)
		lines[l.params] = l
	}
	handle := state.NewHandle(startFunc(lines))
	go state.Loop(handle)
	return &Pipe{
		lines:  lines,
		handle: handle,
	}
}

func (l *Line) bind() error {
	pump, sampleRate, numChannels, err := l.Pump()
	if err != nil {
		return fmt.Errorf("pump: %w", err)
	}
	l.numChannels = numChannels
	l.pump = runner.Pump{
		Fn:    pump.Pump,
		Meter: metric.Meter(l.Pump, signal.SampleRate(sampleRate)),
		Hooks: bindHooks(pump.Hooks),
	}

	// bind processors
	for _, processorFn := range l.Processors {
		processor, sampleRate, _, err := processorFn(sampleRate, numChannels)
		if err != nil {
			return fmt.Errorf("processor: %w", err)
		}
		l.processors = append(l.processors,
			runner.Processor{
				Fn:    processor.Process,
				Meter: metric.Meter(processor, signal.SampleRate(sampleRate)),
				Hooks: bindHooks(processor.Hooks),
			},
		)
	}

	// bind sinks
	for _, sinkFn := range l.Sinks {
		sink, err := sinkFn(sampleRate, numChannels)
		if err != nil {
			return fmt.Errorf("sink: %w", err)
		}
		l.sinks = append(l.sinks,
			runner.Sink{
				Fn:    sink.Sink,
				Meter: metric.Meter(sink, signal.SampleRate(sampleRate)),
				Hooks: bindHooks(sink.Hooks),
			},
		)
	}
	return nil
}

func (l *Line) interrupt() []error {
	var errs []error
	if err := l.pump.Hooks.Interrupt.Call(); err != nil {
		errs = append(errs, err)
	}

	for _, proc := range l.processors {
		if err := proc.Hooks.Interrupt.Call(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, sink := range l.sinks {
		if err := sink.Hooks.Interrupt.Call(); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// start starts the execution of pipe.
func startFunc(lines map[chan state.Params]*Line) state.StartFunc {
	return func(bufferSize int, cancel <-chan struct{}, give chan<- chan state.Params) ([]<-chan error, error) {

		var errBind error
		// try to bind all lines
		for _, l := range lines {
			err := l.bind()
			if err != nil {
				errBind = fmt.Errorf("error binding %w", err)
				break
			}
		}

		// interrupt if error has happened
		if errBind != nil {
			var errsInterrupt []error
			for _, l := range lines {
				if errs := l.interrupt(); errs != nil {
					errsInterrupt = append(errsInterrupt, errs...)
				}
			}
			return nil, fmt.Errorf("%w\n%w", errBind, flatenErrors(errsInterrupt))
		}

		// start all runners
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for params, l := range lines {
			p := pool.New(l.numChannels, bufferSize)
			// start pump
			out, errs := l.pump.Run(p, cancel, give, params)
			errcList = append(errcList, errs)

			// start chained processesing
			for _, proc := range l.processors {
				out, errs = proc.Run(cancel, out)
				errcList = append(errcList, errs)
			}

			sinkErrcList := runner.Broadcast(p, l.sinks, cancel, out)
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

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
// func (p *Pipe) Push(id string, paramFuncs ...func()) {
// 	p.handle.Push(state.Params{id: paramFuncs})
// }

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

// bindHooks of component.
func bindHooks(h Hooks) runner.Hooks {
	return runner.Hooks{
		Flush:     runner.Hook(h.Flush),
		Interrupt: runner.Hook(h.Interrupt),
		Reset:     runner.Hook(h.Reset),
	}
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

package pipe

import (
	"context"
	"fmt"

	"github.com/pipelined/signal"

	"github.com/pipelined/pipe/internal/pool"
	"github.com/pipelined/pipe/internal/runner"
	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/pipe/metric"
)

// Pipe controls the execution of multiple chained lines. Lines might be chained
// through components, mixer for example.  If lines are not chained, they must be
// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
type Pipe struct {
	h                *state.Handle
	lines            map[*Line]string  // map pipe to chain id
	chains           map[string]chain  // map chain id to chain
	chainByComponent map[string]string // map component id to chain id
}

// chain is a runtime chain of the pipeline.
type chain struct {
	uid         string
	sampleRate  signal.SampleRate
	numChannels int
	pump        runner.Pump
	processors  []runner.Processor
	sinks       []runner.Sink
	components  map[interface{}]string
	take        chan runner.Message // emission of messages
	params      state.Params
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ls ...*Line) (*Pipe, error) {
	lines := make(map[*Line]string)
	chains := make(map[string]chain)
	chainByComponent := make(map[string]string)
	for _, p := range ls {
		// bind all lines
		c, err := bindLine(p)
		if err != nil {
			return nil, fmt.Errorf("error binding line: %w", err)
		}
		// map pipe to chain id
		lines[p] = c.uid
		// map chains
		chains[c.uid] = c
		for _, componentID := range c.components {
			chainByComponent[componentID] = c.uid
		}
	}

	p := &Pipe{
		lines:            lines,
		chains:           chains,
		chainByComponent: chainByComponent,
	}
	p.h = state.NewHandle(start(p), newMessage(p), pushParams(p))
	go state.Loop(p.h)
	return p, nil
}

func bindLine(p *Line) (chain, error) {
	components := make(map[interface{}]string)
	pipeID := newUID()
	// bind pump
	pumpFn, sampleRate, numChannels, err := p.Pump.Pump(pipeID)
	if err != nil {
		return chain{}, fmt.Errorf("pump: %w", err)
	}
	pumpRunner := runner.Pump{
		ID:    newUID(),
		Fn:    runner.PumpFunc(pumpFn),
		Meter: metric.Meter(p.Pump, signal.SampleRate(sampleRate)),
		Hooks: BindHooks(p.Pump),
	}
	components[p.Pump] = pumpRunner.ID

	// bind processors
	processorRunners := make([]runner.Processor, 0, len(p.Processors))
	for _, proc := range p.Processors {
		processFn, err := proc.Process(pipeID, sampleRate, numChannels)
		if err != nil {
			return chain{}, fmt.Errorf("processor: %w", err)
		}
		processorRunner := runner.Processor{
			ID:    newUID(),
			Fn:    runner.ProcessFunc(processFn),
			Meter: metric.Meter(proc, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(proc),
		}
		processorRunners = append(processorRunners, processorRunner)
		components[proc] = processorRunner.ID
	}

	// bind sinks
	sinkRunners := make([]runner.Sink, 0, len(p.Sinks))
	for _, sink := range p.Sinks {
		sinkFn, err := sink.Sink(pipeID, sampleRate, numChannels)
		if err != nil {
			return chain{}, fmt.Errorf("sink: %w", err)
		}
		sinkRunner := runner.Sink{
			ID:    newUID(),
			Fn:    runner.SinkFunc(sinkFn),
			Meter: metric.Meter(sink, signal.SampleRate(sampleRate)),
			Hooks: BindHooks(sink),
		}
		sinkRunners = append(sinkRunners, sinkRunner)
		components[sink] = sinkRunner.ID
	}
	return chain{
		uid:         pipeID,
		sampleRate:  sampleRate,
		numChannels: numChannels,
		pump:        pumpRunner,
		processors:  processorRunners,
		sinks:       sinkRunners,
		take:        make(chan runner.Message),
		components:  components,
		params:      make(map[string][]func()),
	}, nil
}

// ComponentID finds id of the component within network.
func (p *Pipe) ComponentID(component interface{}) (id string, ok bool) {
	for _, c := range p.chains {
		if id, ok = c.components[component]; ok {
			break
		}
	}
	return id, ok
}

// start starts the execution of pipe.
func start(p *Pipe) state.StartFunc {
	return func(bufferSize int, cancel <-chan struct{}, give chan<- string) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range p.chains {
			p := pool.New(c.numChannels, bufferSize)
			// start pump
			out, errs := c.pump.Run(p, c.uid, c.pump.ID, cancel, give, c.take)
			errcList = append(errcList, errs)

			// start chained processesing
			for _, proc := range c.processors {
				out, errs = proc.Run(c.uid, proc.ID, cancel, out)
				errcList = append(errcList, errs)
			}

			sinkErrcList := runner.Broadcast(p, c.uid, c.sinks, cancel, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList
	}
}

// newMessage creates a new message with cached Params.
// if new Params are pushed into pipe - next message will contain them.
func newMessage(p *Pipe) state.NewMessageFunc {
	return func(pipeID string) {
		c := p.chains[pipeID]
		m := runner.Message{PipeID: c.uid}
		if len(c.params) > 0 {
			m.Params = c.params
			c.params = make(map[string][]func())
		}
		c.take <- m
	}
}

func pushParams(p *Pipe) state.PushParamsFunc {
	return func(params state.Params) {
		for id, param := range params {
			chainID := p.chainByComponent[id]
			chain := p.chains[chainID]
			chain.params = chain.params.Append(map[string][]func(){id: param})
		}
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
	return p.h.Stop()
}

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Push(id string, paramFuncs ...func()) {
	p.h.Push(state.Params{id: paramFuncs})
}

package pipe

import (
	"context"

	"github.com/pipelined/signal"

	"github.com/pipelined/pipe/internal/runner"
	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/pipe/metric"
)

// L is a pipe(L)ine. It controls the execution of multiple chained pipes.
// If pipes are not chained, they must be controlled by separate (L)ines.
// Use Line constructor to instantiate new pipelines.
type L struct {
	h                *state.Handle
	pipes            map[*Pipe]string  // map pipe to chain id
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
	takec       chan runner.Message // emission of messages
	params      state.Params
}

// Line creates a new pipeline.
// Returned line is in Ready state.
func Line(ps ...*Pipe) (*L, error) {
	pipes := make(map[*Pipe]string)
	chains := make(map[string]chain)
	chainByComponent := make(map[string]string)
	for _, p := range ps {
		// bind all pipes
		c, err := bindPipe(p)
		if err != nil {
			return nil, err
		}
		// map pipe to chain id
		pipes[p] = c.uid
		// map chains
		chains[c.uid] = c
		for _, componentID := range c.components {
			chainByComponent[componentID] = c.uid
		}
	}

	l := &L{
		pipes:            pipes,
		chains:           chains,
		chainByComponent: chainByComponent,
	}
	l.h = state.NewHandle(start(l), newMessage(l), pushParams(l))
	go state.Loop(l.h, state.Ready)
	return l, nil
}

func bindPipe(p *Pipe) (chain, error) {
	components := make(map[interface{}]string)
	pipeID := newUID()
	// bind pump
	pumpFn, sampleRate, numChannels, err := p.Pump.Pump(pipeID)
	if err != nil {
		return chain{}, err
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
			return chain{}, err
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
			return chain{}, err
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
		takec:       make(chan runner.Message),
		components:  components,
		params:      make(map[string][]func()),
	}, nil
}

// ComponentID finds id of the component within network.
func (l *L) ComponentID(component interface{}) (id string, ok bool) {
	for _, c := range l.chains {
		if id, ok = c.components[component]; ok {
			break
		}
	}
	return id, ok
}

// start starts the execution of pipe.
func start(l *L) state.StartFunc {
	return func(bufferSize int, cancelc <-chan struct{}, givec chan<- string) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range l.chains {
			// start pump
			out, errc := c.pump.Run(bufferSize, c.numChannels, c.uid, c.pump.ID, cancelc, givec, c.takec)
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range c.processors {
				out, errc = proc.Run(c.uid, proc.ID, cancelc, out)
				errcList = append(errcList, errc)
			}

			sinkErrcList := runner.Broadcast(c.uid, c.sinks, cancelc, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList
	}
}

// newMessage creates a new message with cached Params.
// if new Params are pushed into pipe - next message will contain them.
func newMessage(l *L) state.NewMessageFunc {
	return func(pipeID string) {
		c := l.chains[pipeID]
		m := runner.Message{SourceID: c.uid}
		if len(c.params) > 0 {
			m.Params = c.params
			c.params = make(map[string][]func())
		}
		c.takec <- m
	}
}

func pushParams(l *L) state.PushParamsFunc {
	return func(params state.Params) {
		for id, p := range params {
			chainID := l.chainByComponent[id]
			chain := l.chains[chainID]
			chain.params = chain.params.Append(map[string][]func(){id: p})
		}
	}
}

// Run sends a run event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback channel is closed when Ready state is reached or context is cancelled.
func (l *L) Run(ctx context.Context, bufferSize int) chan error {
	errc := make(chan error, 1)
	l.h.Eventc <- state.Run{
		Context:    ctx,
		BufferSize: bufferSize,
		Feedback:   errc,
	}
	return errc
}

// Pause sends a pause event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Paused state is reached.
func (l *L) Pause() chan error {
	errc := make(chan error, 1)
	l.h.Eventc <- state.Pause{
		Feedback: errc,
	}
	return errc
}

// Resume sends a resume event into handle.
// Calling this method after handle is closed causes a panic.
// Feedback is closed when Ready state is reached.
func (l *L) Resume() chan error {
	errc := make(chan error, 1)
	l.h.Eventc <- state.Resume{
		Feedback: errc,
	}
	return errc
}

// Close must be called to clean up handle's resources.
// Feedback is closed when line is done.
func (l *L) Close() chan error {
	errc := make(chan error, 1)
	l.h.Eventc <- state.Close{
		Feedback: errc,
	}
	return errc
}

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
func (l *L) Push(id string, paramFuncs ...func()) {
	l.h.Paramc <- map[string][]func(){id: paramFuncs}
}

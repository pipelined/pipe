package pipe

import (
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
	uid        string
	sampleRate int
	pump       *runner.Pump
	processors []*runner.Processor
	sinks      []*runner.Sink
	components map[interface{}]string
	takec      chan runner.Message // emission of messages
	params     state.Params
}

// Line creates a new pipeline.
// Returned net is in Ready state.
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
	pumpRunner := &runner.Pump{
		Fn:    pumpFn,
		Meter: metric.Meter(p.Pump, sampleRate),
		Hooks: runner.BindHooks(p.Pump),
	}
	components[p.Pump] = newUID()

	// bind processors
	processorRunners := make([]*runner.Processor, 0, len(p.Processors))
	for _, proc := range p.Processors {
		processFn, err := proc.Process(pipeID, sampleRate, numChannels)
		if err != nil {
			return chain{}, err
		}
		processorRunners = append(processorRunners, &runner.Processor{
			Fn:    processFn,
			Meter: metric.Meter(proc, sampleRate),
			Hooks: runner.BindHooks(proc),
		})
		components[proc] = newUID()
	}

	// bind sinks
	sinkRunners := make([]*runner.Sink, 0, len(p.Sinks))
	for _, sink := range p.Sinks {
		sinkFn, err := sink.Sink(pipeID, sampleRate, numChannels)
		if err != nil {
			return chain{}, err
		}
		sinkRunners = append(sinkRunners, &runner.Sink{
			Fn:    sinkFn,
			Meter: metric.Meter(sink, sampleRate),
			Hooks: runner.BindHooks(sink),
		})
		components[sink] = newUID()
	}
	return chain{
		uid:        pipeID,
		sampleRate: sampleRate,
		pump:       pumpRunner,
		processors: processorRunners,
		sinks:      sinkRunners,
		takec:      make(chan runner.Message),
		components: components,
		params:     make(map[string][]func()),
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
	return func(bufferSize int, cancelc chan struct{}, givec chan<- string) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range l.chains {
			// start pump
			componentID := c.components[c.pump]
			out, errc := c.pump.Run(bufferSize, c.uid, componentID, cancelc, givec, c.takec)
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range c.processors {
				componentID = c.components[proc]
				// meter := l.metric.Meter(componentID, l.sampleRate)
				out, errc = proc.Run(c.uid, componentID, cancelc, out)
				errcList = append(errcList, errc)
			}

			sinkErrcList := broadcastToSinks(c.sampleRate, c, cancelc, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList
	}
}

// broadcastToSinks passes messages to all sinks.
func broadcastToSinks(sampleRate int, c chain, cancelc chan struct{}, in <-chan runner.Message) []<-chan error {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(c.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan runner.Message, len(c.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan runner.Message)
	}

	//start broadcast
	for i, s := range c.sinks {
		componentID := c.components[s]
		// meter := l.metric.Meter(componentID, l.sampleRate)
		errc := s.Run(c.uid, componentID, cancelc, broadcasts[i])
		errcList = append(errcList, errc)
	}

	go func() {
		//close broadcasts on return
		defer func() {
			for i := range broadcasts {
				close(broadcasts[i])
			}
		}()
		for msg := range in {
			for i := range broadcasts {
				m := runner.Message{
					SourceID: msg.SourceID,
					Buffer:   msg.Buffer,
					Params:   msg.Params.Detach(c.components[c.sinks[i]]),
				}
				select {
				case broadcasts[i] <- m:
				case <-cancelc:
					return
				}
			}
		}
	}()

	return errcList
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
// Feedback is closed when Ready state is reached.
func (l *L) Run(bufferSize int) chan error {
	errc := make(chan error, 1)
	l.h.Eventc <- state.Run{
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

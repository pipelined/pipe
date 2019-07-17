package pipe

import (
	"github.com/pipelined/pipe/internal/runner"
	"github.com/pipelined/pipe/internal/state"
	"github.com/pipelined/pipe/metric"
)

// Net controls the execution of pipes.
type Net struct {
	*state.Handle
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

// Network creates a new net.
// Returned net is in Ready state.
func Network(ps ...*Pipe) (*Net, error) {
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

	n := &Net{
		pipes:            pipes,
		chains:           chains,
		chainByComponent: chainByComponent,
	}
	n.Handle = state.NewHandle(start(n), newMessage(n), pushParams(n))
	go state.Loop(n.Handle)
	return n, nil
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
func (n *Net) ComponentID(component interface{}) (id string, ok bool) {
	for _, c := range n.chains {
		if id, ok = c.components[component]; ok {
			break
		}
	}
	return id, ok
}

// start starts the execution of pipe.
func start(n *Net) state.StartFunc {
	return func(bufferSize int, cancelc chan struct{}, provide chan<- string) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range n.chains {
			// start pump
			componentID := c.components[c.pump]
			out, errc := c.pump.Run(bufferSize, c.uid, componentID, cancelc, provide, c.takec)
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range c.processors {
				componentID = c.components[proc]
				// meter := n.metric.Meter(componentID, n.sampleRate)
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
		// meter := n.metric.Meter(componentID, n.sampleRate)
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
func newMessage(n *Net) state.NewMessageFunc {
	return func(pipeID string) {
		c := n.chains[pipeID]
		m := runner.Message{SourceID: c.uid}
		if len(c.params) > 0 {
			m.Params = c.params
			c.params = make(map[string][]func())
		}
		c.takec <- m
	}
}

func pushParams(n *Net) state.PushParamsFunc {
	return func(componentID string, params state.Params) {
		chainID := n.chainByComponent[componentID]
		chain := n.chains[chainID]
		chain.params = chain.params.Merge(params)
		n.chains[chainID] = chain
	}
}

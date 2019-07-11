package pipe

import (
	"github.com/pipelined/pipe/internal/state"
)

// Metric measures performance of component.
type Metric interface {
	AddComponent(componentID string, sampleRate int) ComponentMetric
}

// ComponentMetric captures metrics after each message being processed by component.
type ComponentMetric interface {
	Message(size int) ComponentMetric
}

// Net controls the execution of pipes.
type Net struct {
	*state.Handle
	bufferSize    int
	chains        map[string]chain  // map chain id to chain
	componentNets map[string]string // map component id to chain id
	pipes         []Pipe
	metric        Metric
	log           Logger
}

// chain is a runtime chain of the pipeline.
type chain struct {
	uid        string
	pump       *pumpRunner
	processors []*processRunner
	sinks      []*sinkRunner
	components map[interface{}]string
	consume    chan message // emission of messages
	params     state.Params
}

// Option provides a way to set functional parameters to the Net.
type Option func(*Net) error

// Network creates a new net.
// Returned net is in Ready state.
func Network(pipes ...Pipe) (*Net, error) {
	chains := make(map[string]chain)
	componentNets := make(map[string]string)
	for _, p := range pipes {
		// bind all pipes
		c, err := bindPipe(p)
		if err != nil {
			return nil, err
		}
		chains[c.uid] = c
		for _, componentID := range c.components {
			componentNets[componentID] = c.uid
		}
	}

	f := &Net{
		chains: chains,
		log:    defaultLogger,
	}
	f.Handle = state.NewHandle(start(f), newMessage(f), pushParams(f))
	go state.Loop(f.Handle)
	return f, nil
}

func bindPipe(p Pipe) (chain, error) {
	components := make(map[interface{}]string)
	uid := newUID()
	// newPumpRunner should not be created here.
	pumpRunner, sampleRate, numChannels, err := bindPump(uid, p.Pump)
	if err != nil {
		return chain{}, err
	}
	components[pumpRunner] = newUID()

	processorRunners := make([]*processRunner, 0, len(p.Processors))
	for _, proc := range p.Processors {
		r, err := bindProcessor(uid, sampleRate, numChannels, proc)
		if err != nil {
			return chain{}, err
		}
		processorRunners = append(processorRunners, r)
		components[r] = newUID()
	}
	// create all runners
	sinkRunners := make([]*sinkRunner, 0, len(p.Sinks))
	for _, s := range p.Sinks {
		// sinkRunner should not be created here.
		r, err := bindSink(uid, sampleRate, numChannels, s)
		if err != nil {
			return chain{}, err
		}
		sinkRunners = append(sinkRunners, r)
		components[r] = newUID()
	}
	return chain{
		uid:        uid,
		pump:       pumpRunner,
		processors: processorRunners,
		sinks:      sinkRunners,
		consume:    make(chan message),
		components: components,
		params:     make(map[string][]func()),
		// feedback:   make(map[string][]func()),
	}, nil
}

// WithLogger sets logger to Pipe. If this option is not provided, silent logger is used.
func WithLogger(logger Logger) Option {
	return func(f *Net) error {
		f.log = logger
		return nil
	}
}

// WithMetric adds meterics for this pipe and all components.
func WithMetric(m Metric) Option {
	return func(f *Net) error {
		f.metric = m
		return nil
	}
}

// start starts the execution of pipe.
func start(f *Net) state.StartFunc {
	return func(bufferSize int, cancelc chan struct{}, provide chan<- string) []<-chan error {
		// error channel for each component
		errcList := make([]<-chan error, 0)
		for _, c := range f.chains {
			// start pump
			componentID := c.components[c.pump]
			out, errc := c.pump.run(bufferSize, c.uid, componentID, cancelc, provide, c.consume, nil)
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range c.processors {
				componentID = c.components[proc]
				// meter := f.metric.Meter(componentID, f.sampleRate)
				out, errc = proc.run(c.uid, componentID, cancelc, out, nil)
				errcList = append(errcList, errc)
			}

			sinkErrcList := broadcastToSinks(c, cancelc, out)
			errcList = append(errcList, sinkErrcList...)
		}
		return errcList
	}
}

// broadcastToSinks passes messages to all sinks.
func broadcastToSinks(c chain, cancelc chan struct{}, in <-chan message) []<-chan error {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(c.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan message, len(c.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan message)
	}

	//start broadcast
	for i, s := range c.sinks {
		componentID := c.components[s]
		// meter := f.metric.Meter(componentID, f.sampleRate)
		errc := s.run(c.uid, componentID, cancelc, broadcasts[i], nil)
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
				m := message{
					sourceID: msg.sourceID,
					buffer:   msg.buffer,
					params:   msg.params.Detach(c.components[c.sinks[i]]),
					// Feedback: msg.Feedback.Detach(c.components[c.sinks[i]]),
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
func newMessage(f *Net) state.NewMessageFunc {
	return func(pipeID string) {
		c := f.chains[pipeID]
		m := message{sourceID: c.uid}
		if len(c.params) > 0 {
			m.params = c.params
			c.params = make(map[string][]func())
		}
		c.consume <- m
	}
}

func pushParams(f *Net) state.PushParamsFunc {
	return func(componentID string, params state.Params) {
		chainID := f.componentNets[componentID]
		chain := f.chains[chainID]
		chain.params = chain.params.Merge(params)
		f.chains[chainID] = chain
	}
}

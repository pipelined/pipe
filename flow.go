package pipe

import (
	"sync"
)

type Metric interface {
	AddComponent(componentID string, sampleRate int) ComponentMetric
}

type ComponentMetric interface {
	Message(size int) ComponentMetric
}

// Flow controls the execution of pipes.
type Flow struct {
	// sampleRate  int
	// numChannels int
	bufferSize int

	chains map[string]chain // keep chain of the whole pipe
	pipes  []Pipe

	metric   Metric
	params   params            //cahced params
	feedback params            //cached feedback
	errc     chan error        // errors channel
	events   chan eventMessage // event channel
	cancel   chan struct{}     // cancellation channel
	provide  chan string       // ask for new message request for the chain

	// components is a map of components and their ids.
	// this makes the case when one component is used across multiple pipes to have
	// different id in different pipes.
	// this will be a part of runner later.

	log Logger
}

// chain is a runtime chain of the pipeline.
type chain struct {
	uid        string
	pump       *pumpRunner
	processors []*processRunner
	sinks      []*sinkRunner
	components map[interface{}]string
	consume    chan message // emission of messages
}

// Option provides a way to set functional parameters to flow.
type Option func(*Flow) error

// New creates a new flow and applies provided options.
// Returned flow is in Ready state.
func New(bufferSize int, p Pipe) (*Flow, error) {
	chains := make(map[string]chain)
	c, err := bindPipe(bufferSize, p)
	if err != nil {
		return nil, err
	}
	chains[c.uid] = c
	f := &Flow{
		chains:     chains,
		bufferSize: bufferSize,
		log:        defaultLogger,
		params:     make(map[string][]func()),
		feedback:   make(map[string][]func()),
		events:     make(chan eventMessage, 1),
		provide:    make(chan string),
	}
	go loop(f)
	return f, nil
}

func bindPipe(bufferSize int, p Pipe) (chain, error) {
	components := make(map[interface{}]string)
	uid := newUID()
	// newPumpRunner should not be created here.
	pumpRunner, sampleRate, numChannels, err := bindPump(uid, bufferSize, p.Pump)
	if err != nil {
		return chain{}, err
	}
	components[pumpRunner] = newUID()

	processorRunners := make([]*processRunner, 0, len(p.Processors))
	for _, proc := range p.Processors {
		r, err := bindProcessor(uid, sampleRate, numChannels, bufferSize, proc)
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
		r, err := bindSink(uid, sampleRate, numChannels, bufferSize, s)
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
	}, nil
}

// WithLogger sets logger to Pipe. If this option is not provided, silent logger is used.
func WithLogger(logger Logger) Option {
	return func(f *Flow) error {
		f.log = logger
		return nil
	}
}

// WithMetric adds meterics for this pipe and all components.
func WithMetric(m Metric) Option {
	return func(f *Flow) error {
		f.metric = m
		return nil
	}
}

// WithPump sets pump to Pipe.
// func WithPump(pump Pump) Option {
// 	return func(f *Flow) error {
// 		// newPumpRunner should not be created here.
// 		r, sampleRate, numChannels, err := bindPump(f.uid, f.bufferSize, pump)
// 		if err != nil {
// 			return err
// 		}
// 		f.pump = r
// 		f.sampleRate = sampleRate
// 		f.numChannels = numChannels
// 		addComponent(f, r)
// 		return nil
// 	}
// }

// WithProcessors sets processors to Pipe.
// func WithProcessors(processors ...Processor) Option {
// 	return func(f *Flow) error {
// 		runners := make([]*processRunner, 0, len(processors))
// 		for _, proc := range processors {
// 			// processRunner should not be created here.
// 			r, err := bindProcessor(f.uid, f.sampleRate, f.numChannels, f.bufferSize, proc)
// 			if err != nil {
// 				return err
// 			}
// 			runners = append(runners, r)
// 		}
// 		f.processors = append(f.processors, runners...)
// 		// add all processors to params map
// 		for _, proc := range runners {
// 			addComponent(f, proc)
// 		}
// 		return nil
// 	}
// }

// WithSinks sets sinks to Pipe.
// func WithSinks(sinks ...Sink) Option {
// 	return func(f *Flow) error {
// 		// create all runners
// 		runners := make([]*sinkRunner, 0, len(sinks))
// 		for _, s := range sinks {
// 			// sinkRunner should not be created here.
// 			r, err := bindSink(f.uid, f.sampleRate, f.numChannels, f.bufferSize, s)
// 			if err != nil {
// 				return err
// 			}
// 			runners = append(runners, r)
// 		}
// 		f.sinks = append(f.sinks, runners...)
// 		// add all sinks to params map
// 		for _, s := range runners {
// 			addComponent(f, s)
// 		}
// 		return nil
// 	}
// }

// ComponentID returns component id.
// func (f *Flow) ComponentID(component interface{}) string {
// 	if f == nil || f.components == nil {
// 		return ""
// 	}
// 	return f.components[component]
// }

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
// func (f *Flow) Push(component interface{}, paramFuncs ...func()) {
// 	var componentID string
// 	var ok bool
// 	if componentID, ok = f.components[component]; !ok && len(paramFuncs) == 0 {
// 		return
// 	}
// 	params := params(make(map[string][]func()))
// 	f.events <- eventMessage{
// 		event:  push,
// 		params: params.add(componentID, paramFuncs...),
// 	}
// }

// addComponent adds new uid for components map.
// func addComponent(f *Flow, c interface{}) {
// 	uid := newUID()
// 	f.components[c] = uid
// }

// start starts the execution of pipe.
func start(c chain, cancelc chan struct{}, provide chan<- string) []<-chan error {
	// error channel for each component
	errcList := make([]<-chan error, 0, 1+len(c.processors)+len(c.sinks))
	// start pump
	componentID := c.components[c.pump]
	out, errc := c.pump.run(c.uid, componentID, cancelc, provide, c.consume, nil)
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
	return errcList
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
					params:   msg.params.detach(c.components[c.sinks[i]]),
					feedback: msg.feedback.detach(c.components[c.sinks[i]]),
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

// merge error channels from all components into one.
func mergeErrors(errcList ...<-chan error) (errc chan error) {
	var wg sync.WaitGroup
	errc = make(chan error, len(errcList))

	//function to wait for error channel
	output := func(ec <-chan error) {
		for e := range ec {
			errc <- e
		}
		wg.Done()
	}
	wg.Add(len(errcList))
	for _, ec := range errcList {
		go output(ec)
	}

	//wait and close out
	go func() {
		wg.Wait()
		close(errc)
	}()

	return
}

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
func New(p Pipe) (*Flow, error) {
	chains := make(map[string]chain)
	c, err := bindPipe(p)
	if err != nil {
		return nil, err
	}
	chains[c.uid] = c
	f := &Flow{
		chains:   chains,
		log:      defaultLogger,
		params:   make(map[string][]func()),
		feedback: make(map[string][]func()),
		events:   make(chan eventMessage, 1),
		provide:  make(chan string),
	}
	go loop(f)
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

// start starts the execution of pipe.
func start(bufferSize int, c chain, cancelc chan struct{}, provide chan<- string) []<-chan error {
	// error channel for each component
	errcList := make([]<-chan error, 0, 1+len(c.processors)+len(c.sinks))
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

package pipe

import (
	"sync"

	"github.com/pipelined/pipe/metric"
)

// Flow controls the execution of pipes.
type Flow struct {
	uid         string
	sampleRate  int
	numChannels int
	bufferSize  int
	pump        *pumpRunner
	processors  []*processRunner
	sinks       []*sinkRunner

	metric   *metric.Metric
	params   params            //cahced params
	feedback params            //cached feedback
	errc     chan error        // errors channel
	events   chan eventMessage // event channel
	cancel   chan struct{}     // cancellation channel

	provide chan struct{} // ask for new message request
	consume chan message  // emission of messages

	// components is a map of components and their ids.
	// this makes the case when one component is used across multiple pipes to have
	// different id in different pipes.
	// this will be a part of runner later.
	components map[interface{}]string
	log        Logger
}

// Option provides a way to set functional parameters to flow.
type Option func(*Flow) error

// New creates a new flow and applies provided options.
// Returned flow is in Ready state.
func New(bufferSize int, options ...Option) (*Flow, error) {
	f := &Flow{
		uid:        newUID(),
		bufferSize: bufferSize,
		log:        defaultLogger,
		processors: make([]*processRunner, 0),
		sinks:      make([]*sinkRunner, 0),
		params:     make(map[string][]func()),
		feedback:   make(map[string][]func()),
		events:     make(chan eventMessage, 1),
		provide:    make(chan struct{}),
		consume:    make(chan message),
		components: make(map[interface{}]string),
	}
	for _, option := range options {
		err := option(f)
		if err != nil {
			return nil, err
		}
	}
	go loop(f)
	return f, nil
}

// WithLogger sets logger to Pipe. If this option is not provided, silent logger is used.
func WithLogger(logger Logger) Option {
	return func(f *Flow) error {
		f.log = logger
		return nil
	}
}

// WithMetric adds meterics for this pipe and all components.
func WithMetric(m *metric.Metric) Option {
	return func(f *Flow) error {
		f.metric = m
		return nil
	}
}

// WithPump sets pump to Pipe.
func WithPump(pump Pump) Option {
	return func(f *Flow) error {
		// newPumpRunner should not be created here.
		r, sampleRate, numChannels, err := newPumpRunner(f.uid, f.bufferSize, pump)
		if err != nil {
			return err
		}
		f.pump = r
		f.sampleRate = sampleRate
		f.numChannels = numChannels
		addComponent(f, pump)
		return nil
	}
}

// WithProcessors sets processors to Pipe.
func WithProcessors(processors ...Processor) Option {
	return func(f *Flow) error {
		runners := make([]*processRunner, 0, len(processors))
		for _, proc := range processors {
			// processRunner should not be created here.
			r, err := newProcessRunner(f.uid, f.sampleRate, f.numChannels, f.bufferSize, proc)
			if err != nil {
				return err
			}
			runners = append(runners, r)
		}
		f.processors = append(f.processors, runners...)
		// add all processors to params map
		for _, proc := range processors {
			addComponent(f, proc)
		}
		return nil
	}
}

// WithSinks sets sinks to Pipe.
func WithSinks(sinks ...Sink) Option {
	return func(f *Flow) error {
		// create all runners
		runners := make([]*sinkRunner, 0, len(sinks))
		for _, s := range sinks {
			// sinkRunner should not be created here.
			r, err := newSinkRunner(f.uid, f.sampleRate, f.numChannels, f.bufferSize, s)
			if err != nil {
				return err
			}
			runners = append(runners, r)
		}
		f.sinks = append(f.sinks, runners...)
		// add all sinks to params map
		for _, s := range sinks {
			addComponent(f, s)
		}
		return nil
	}
}

// ComponentID returns component id.
func (f *Flow) ComponentID(component interface{}) string {
	if f == nil || f.components == nil {
		return ""
	}
	return f.components[component]
}

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
func (f *Flow) Push(component interface{}, paramFuncs ...func()) {
	var componentID string
	var ok bool
	if componentID, ok = f.components[component]; !ok && len(paramFuncs) == 0 {
		return
	}
	params := params(make(map[string][]func()))
	f.events <- eventMessage{
		event:  push,
		params: params.add(componentID, paramFuncs...),
	}
}

// addComponent adds new uid for components map.
func addComponent(f *Flow, c interface{}) {
	uid := newUID()
	f.components[c] = uid
}

// start starts the execution of pipe.
func start(f *Flow) {
	f.cancel = make(chan struct{})
	errcList := make([]<-chan error, 0, 1+len(f.processors)+len(f.sinks))
	// start pump
	componentID := f.components[f.pump.Pump]
	meter := f.metric.Meter(componentID, f.sampleRate)
	out, errc := f.pump.run(f.uid, componentID, f.cancel, f.provide, f.consume, meter)
	errcList = append(errcList, errc)

	// start chained processesing
	for _, proc := range f.processors {
		componentID = f.components[proc.Processor]
		meter := f.metric.Meter(componentID, f.sampleRate)
		out, errc = proc.run(f.uid, componentID, f.cancel, out, meter)
		errcList = append(errcList, errc)
	}

	sinkErrcList := broadcastToSinks(f, out)
	errcList = append(errcList, sinkErrcList...)
	f.errc = mergeErrors(errcList...)
}

// broadcastToSinks passes messages to all sinks.
func broadcastToSinks(f *Flow, in <-chan message) []<-chan error {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(f.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan message, len(f.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan message)
	}

	//start broadcast
	for i, s := range f.sinks {
		componentID := f.components[s.Sink]
		meter := f.metric.Meter(componentID, f.sampleRate)
		errc := s.run(f.uid, componentID, f.cancel, broadcasts[i], meter)
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
					params:   msg.params.detach(f.components[f.sinks[i].Sink]),
					feedback: msg.feedback.detach(f.components[f.sinks[i].Sink]),
				}
				select {
				case broadcasts[i] <- m:
				case <-f.cancel:
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

// newMessage creates a new message with cached params.
// if new params are pushed into pipe - next message will contain them.
func newMessage(f *Flow) message {
	m := message{sourceID: f.uid}
	if len(f.params) > 0 {
		m.params = f.params
		f.params = make(map[string][]func())
	}
	if len(f.feedback) > 0 {
		m.feedback = f.feedback
		f.feedback = make(map[string][]func())
	}
	return m
}

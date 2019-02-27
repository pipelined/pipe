package pipe

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pipelined/pipe/metric"

	"github.com/pipelined/signal"
	"github.com/rs/xid"
)

// Pump is a source of samples. Pump method returns a new buffer with signal data.
// Implentetions should use next error conventions:
// 		- nil if a full buffer was read;
// 		- io.EOF if no data was read;
// 		- io.ErrUnexpectedEOF if not a full buffer was read.
// The latest case means that pump executed as expected, but not enough data was available.
// This incomplete buffer still will be sent further and pump will be finished gracefully.
type Pump interface {
	Pump(pipeID string, bufferSize int) (func() ([][]float64, error), int, int, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) ([][]float64, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error)
}

// Logger is a global interface for pipe loggers
type Logger interface {
	Debug(...interface{})
	Info(...interface{})
}

// newUID returns new unique id value.
func newUID() string {
	return xid.New().String()
}

// Message is a main structure for pipe transport
type message struct {
	buffer   signal.Float64 // Buffer of message
	params                  // params for pipe
	sourceID string         // ID of pipe which spawned this message.
	feedback params         //feedback are params applied after processing happened
}

// params represent a set of parameters mapped to ID of their receivers.
type params map[string][]func()

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	uid         string
	name        string
	sampleRate  int
	numChannels int
	bufferSize  int

	pump       *pumpRunner
	processors []*processRunner
	sinks      []*sinkRunner

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

// Option provides a way to set functional parameters to pipe.
type Option func(p *Pipe) error

// ErrInvalidState is returned if pipe method cannot be executed at this moment.
var ErrInvalidState = errors.New("invalid state")

// New creates a new pipe and applies provided options.
// Returned pipe is in Ready state.
func New(bufferSize int, options ...Option) (*Pipe, error) {
	p := &Pipe{
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
		err := option(p)
		if err != nil {
			return nil, err
		}
	}
	go p.loop()
	return p, nil
}

// WithLogger sets logger to Pipe. If this option is not provided, silent logger is used.
func WithLogger(logger Logger) Option {
	return func(p *Pipe) error {
		p.log = logger
		return nil
	}
}

// WithName sets name to Pipe.
func WithName(n string) Option {
	return func(p *Pipe) error {
		p.name = n
		return nil
	}
}

// WithMetric adds meterics for this pipe and all components.
func WithMetric(m *metric.Metric) Option {
	return func(p *Pipe) error {
		p.metric = m
		return nil
	}
}

// WithPump sets pump to Pipe.
func WithPump(pump Pump) Option {
	return func(p *Pipe) error {
		uid := newUID()
		p.components[pump] = uid
		r, sampleRate, numChannels, err := newPumpRunner(p.uid, p.bufferSize, pump)
		if err != nil {
			return err
		}
		p.pump = r
		p.sampleRate = sampleRate
		p.numChannels = numChannels
		return nil
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...Processor) Option {
	return func(p *Pipe) error {
		for _, proc := range processors {
			uid := newUID()
			p.components[proc] = uid
			r, err := newProcessRunner(p.uid, p.sampleRate, p.numChannels, p.bufferSize, proc)
			if err != nil {
				return err
			}
			p.processors = append(p.processors, r)
		}
		return nil
	}
}

// WithSinks sets sinks to Pipe.
func WithSinks(sinks ...Sink) Option {
	return func(p *Pipe) error {
		for _, sink := range sinks {
			uid := newUID()
			p.components[sink] = uid
			r, err := newSinkRunner(p.uid, p.sampleRate, p.numChannels, p.bufferSize, sink)
			if err != nil {
				return err
			}
			p.sinks = append(p.sinks, r)
		}
		return nil
	}
}

// Push new params into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Push(component interface{}, paramFuncs ...func()) {
	var componentID string
	var ok bool
	if componentID, ok = p.components[component]; !ok && len(paramFuncs) == 0 {
		return
	}
	params := params(make(map[string][]func()))
	p.events <- eventMessage{
		event:  push,
		params: params.add(componentID, paramFuncs...),
	}
}

// start starts the execution of pipe.
func (p *Pipe) start() {
	p.cancel = make(chan struct{})
	errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
	// start pump
	componentID := p.components[p.pump.Pump]
	meter := p.metric.Meter(componentID, p.sampleRate)
	out, errc := p.pump.run(p.uid, componentID, p.cancel, p.provide, p.consume, meter)
	errcList = append(errcList, errc)

	// start chained processesing
	for _, proc := range p.processors {
		componentID = p.components[proc.Processor]
		meter := p.metric.Meter(componentID, p.sampleRate)
		out, errc = proc.run(p.uid, componentID, p.cancel, out, meter)
		errcList = append(errcList, errc)
	}

	sinkErrcList := p.broadcastToSinks(out)
	errcList = append(errcList, sinkErrcList...)
	p.errc = mergeErrors(errcList...)
}

// broadcastToSinks passes messages to all sinks.
func (p *Pipe) broadcastToSinks(in <-chan message) []<-chan error {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan message)
	}

	//start broadcast
	for i, s := range p.sinks {
		componentID := p.components[s.Sink]
		meter := p.metric.Meter(componentID, p.sampleRate)
		errc := s.run(p.uid, componentID, p.cancel, broadcasts[i], meter)
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
					params:   msg.params.detach(p.components[p.sinks[i].Sink]),
					feedback: msg.feedback.detach(p.components[p.sinks[i].Sink]),
				}
				select {
				case broadcasts[i] <- m:
				case <-p.cancel:
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
func (p *Pipe) newMessage() message {
	m := message{sourceID: p.uid}
	if len(p.params) > 0 {
		m.params = p.params
		p.params = make(map[string][]func())
	}
	if len(p.feedback) > 0 {
		m.feedback = p.feedback
		p.feedback = make(map[string][]func())
	}
	return m
}

// Convert the event to a string.
func (e event) String() string {
	switch e {
	case run:
		return "run"
	case pause:
		return "pause"
	case resume:
		return "resume"
	case push:
		return "params"
	}
	return "unknown"
}

// Convert pipe to string. If name is included if has value.
func (p *Pipe) String() string {
	if p.name == "" {
		return p.uid
	}
	return fmt.Sprintf("%v %v", p.name, p.uid)
}

// add appends a slice of params.
func (p params) add(componentID string, paramFuncs ...func()) params {
	var private []func()
	if _, ok := p[componentID]; !ok {
		private = make([]func(), 0, len(paramFuncs))
	}
	private = append(private, paramFuncs...)

	p[componentID] = private
	return p
}

// applyTo consumes params defined for consumer in this param set.
func (p params) applyTo(id string) {
	if p == nil {
		return
	}
	if params, ok := p[id]; ok {
		for _, param := range params {
			param()
		}
		delete(p, id)
	}
}

// merge two param sets into one.
func (p params) merge(source params) params {
	for newKey, newValues := range source {
		if _, ok := p[newKey]; ok {
			p[newKey] = append(p[newKey], newValues...)
		} else {
			p[newKey] = newValues
		}
	}
	return p
}

func (p params) detach(id string) params {
	if p == nil {
		return nil
	}
	if v, ok := p[id]; ok {
		d := params(make(map[string][]func()))
		d[id] = v
		return d
	}
	return nil
}

// ComponentID returns component id.
func (p *Pipe) ComponentID(component interface{}) string {
	if p == nil || p.components == nil {
		return ""
	}
	return p.components[component]
}

type silentLogger struct{}

func (silentLogger) Debug(args ...interface{}) {}

func (silentLogger) Info(args ...interface{}) {}

var defaultLogger silentLogger

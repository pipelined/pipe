package pipe

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pipelined/phono"
	"github.com/pipelined/phono/log"
)

// Message is a main structure for pipe transport
type message struct {
	phono.Buffer        // Buffer of message
	params              // params for pipe
	sourceID     string // ID of pipe which spawned this message.
	feedback     params //feedback are params applied after processing happened
}

// params represent a set of parameters mapped to ID of their receivers.
type params map[string][]phono.ParamFunc

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	phono.UID
	name       string
	sampleRate phono.SampleRate

	pump       *pumpRunner
	processors []*processRunner
	sinks      []*sinkRunner

	metric   phono.Metric
	params   params            //cahced params
	feedback params            //cached feedback
	errc     chan error        // errors channel
	events   chan eventMessage // event channel
	cancel   chan struct{}     // cancellation channel

	provide chan struct{} // ask for new message request
	consume chan message  // emission of messages

	log log.Logger
}

// Option provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Option func(p *Pipe) error

// ErrInvalidState is returned if pipe method cannot be executed at this moment.
var ErrInvalidState = errors.New("invalid state")

// ErrComponentNoID is used to cause a panic when new component without ID is added to pipe.
var ErrComponentNoID = errors.New("component have no ID value")

// New creates a new pipe and applies provided options.
// Returned pipe is in Ready state.
func New(sampleRate phono.SampleRate, options ...Option) (*Pipe, error) {
	p := &Pipe{
		UID:        phono.NewUID(),
		sampleRate: sampleRate,
		log:        log.GetLogger(),
		processors: make([]*processRunner, 0),
		sinks:      make([]*sinkRunner, 0),
		params:     make(map[string][]phono.ParamFunc),
		feedback:   make(map[string][]phono.ParamFunc),
		events:     make(chan eventMessage, 1),
		provide:    make(chan struct{}),
		consume:    make(chan message),
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

// WithName sets name to Pipe
func WithName(n string) Option {
	return func(p *Pipe) error {
		p.name = n
		return nil
	}
}

// WithMetric adds meterics for this pipe and all components.
func WithMetric(m phono.Metric) Option {
	return func(p *Pipe) error {
		p.metric = m
		return nil
	}
}

// WithPump sets pump to Pipe
func WithPump(pump phono.Pump) Option {
	if pump.ID() == "" {
		panic(ErrComponentNoID)
	}
	return func(p *Pipe) error {
		r, err := newPumpRunner(p.ID(), pump)
		if err != nil {
			return err
		}
		p.pump = r
		return nil
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...phono.Processor) Option {
	for i := range processors {
		if processors[i].ID() == "" {
			panic(ErrComponentNoID)
		}
	}
	return func(p *Pipe) error {
		for _, proc := range processors {
			r, err := newProcessRunner(p.ID(), proc)
			if err != nil {
				return err
			}
			p.processors = append(p.processors, r)
		}
		return nil
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...phono.Sink) Option {
	for _, sink := range sinks {
		if sink.ID() == "" {
			panic(ErrComponentNoID)
		}
	}
	return func(p *Pipe) error {
		for _, sink := range sinks {
			r, err := newSinkRunner(p.ID(), sink)
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
func (p *Pipe) Push(values ...phono.Param) {
	if len(values) == 0 {
		return
	}
	params := params(make(map[string][]phono.ParamFunc))
	p.events <- eventMessage{
		event:  push,
		params: params.add(values...),
	}
}

// start starts the execution of pipe.
func (p *Pipe) start() {
	p.cancel = make(chan struct{})
	errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
	// start pump
	out, errc := p.pump.run(p.cancel, p.ID(), p.provide, p.consume, p.sampleRate, p.metric)
	errcList = append(errcList, errc)

	// start chained processesing
	for _, proc := range p.processors {
		out, errc = proc.run(p.cancel, p.ID(), out, p.sampleRate, p.metric)
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
		errc := s.run(p.cancel, p.ID(), broadcasts[i], p.sampleRate, p.metric)
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
					Buffer:   msg.Buffer,
					params:   msg.params.detach(p.sinks[i].ID()),
					feedback: msg.feedback.detach(p.sinks[i].ID()),
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
	m := message{sourceID: p.ID()}
	if len(p.params) > 0 {
		m.params = p.params
		p.params = make(map[string][]phono.ParamFunc)
	}
	if len(p.feedback) > 0 {
		m.feedback = p.feedback
		p.feedback = make(map[string][]phono.ParamFunc)
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
		return p.ID()
	}
	return fmt.Sprintf("%v %v", p.name, p.ID())
}

// add appends a slice of params.
func (p params) add(params ...phono.Param) params {
	for _, param := range params {
		private, ok := p[param.ID]
		if !ok {
			private = make([]phono.ParamFunc, 0, len(params))
		}
		private = append(private, param.Apply)

		p[param.ID] = private
	}
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
		d := params(make(map[string][]phono.ParamFunc))
		d[id] = v
		return d
	}
	return nil
}

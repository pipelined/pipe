package pipe

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
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

	metrics  map[string]measurable // metrics holds all references to measurable components
	params   params                //cahced params
	feedback params                //cached feedback
	errc     chan error            // errors channel
	events   chan eventMessage     // event channel
	cancel   chan struct{}         // cancellation channel

	provide chan struct{} // ask for new message request
	consume chan message  // emission of messages

	log log.Logger
}

// Option provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Option func(p *Pipe) phono.ParamFunc

// ErrInvalidState is returned if pipe method cannot be executed at this moment.
var ErrInvalidState = errors.New("invalid state")

// ErrComponentNoID is used to cause a panic when new component without ID is added to pipe.
var ErrComponentNoID = errors.New("component have no ID value")

// New creates a new pipe and applies provided options.
// Returned pipe is in Ready state.
func New(sampleRate phono.SampleRate, options ...Option) *Pipe {
	p := &Pipe{
		UID:        phono.NewUID(),
		sampleRate: sampleRate,
		log:        log.GetLogger(),
		processors: make([]*processRunner, 0),
		sinks:      make([]*sinkRunner, 0),
		metrics:    make(map[string]measurable),
		params:     make(map[string][]phono.ParamFunc),
		feedback:   make(map[string][]phono.ParamFunc),
		events:     make(chan eventMessage, 1),
		cancel:     make(chan struct{}),
		provide:    make(chan struct{}),
		consume:    make(chan message),
	}
	for _, option := range options {
		option(p)()
	}
	go p.loop()
	return p
}

// WithName sets name to Pipe
func WithName(n string) Option {
	return func(p *Pipe) phono.ParamFunc {
		return func() error {
			p.name = n
			return nil
		}
	}
}

// WithPump sets pump to Pipe
func WithPump(pump phono.Pump) Option {
	if pump.ID() == "" {
		panic(ErrComponentNoID)
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() error {
			r := &pumpRunner{
				Pump:       pump,
				hooks:      bindHooks(pump),
				measurable: newMetric(pump.ID(), p.sampleRate, counters.pump...),
			}
			p.metrics[r.ID()] = r
			p.pump = r
			return nil
		}
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...phono.Processor) Option {
	for i := range processors {
		if processors[i].ID() == "" {
			panic(ErrComponentNoID)
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() error {
			for i := range processors {
				r := &processRunner{
					Processor:  processors[i],
					hooks:      bindHooks(processors[i]),
					measurable: newMetric(processors[i].ID(), p.sampleRate, counters.processor...),
				}
				p.metrics[r.ID()] = r
				p.processors = append(p.processors, r)
			}
			return nil
		}
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...phono.Sink) Option {
	for i := range sinks {
		if sinks[i].ID() == "" {
			panic(ErrComponentNoID)
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() error {
			for i := range sinks {
				r := &sinkRunner{
					Sink:       sinks[i],
					hooks:      bindHooks(sinks[i]),
					measurable: newMetric(sinks[i].ID(), p.sampleRate, counters.sink...),
				}
				p.metrics[r.ID()] = r
				p.sinks = append(p.sinks, r)
			}
			return nil
		}
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

// Measure returns a buffered channel of Measure.
// Event is pushed into pipe to retrieve metrics. Metric measure is returned immediately if
// state is idle, otherwise it's returned once it's reached destination within pipe.
// Result channel has buffer sized to found components. Order of measures can differ from
// requested order due to pipeline configuration.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Measure(ids ...string) <-chan Measure {
	if len(p.metrics) == 0 {
		return nil
	}
	em := eventMessage{event: measure, params: make(map[string][]phono.ParamFunc)}
	if len(ids) > 0 {
		em.components = make([]string, 0, len(ids))
		// check if passed ids are part of the pipe
		for _, id := range ids {
			_, ok := p.metrics[id]
			if ok {
				em.components = append(em.components, id)
			}
		}
		// no components found
		if len(em.components) == 0 {
			return nil
		}
	} else {
		em.components = make([]string, 0, len(p.metrics))
		// get all if nothing is passed
		for id := range p.metrics {
			em.components = append(em.components, id)
		}
	}

	done := make(chan struct{}, len(em.components)) // done chan to close metrics channel
	mc := make(chan Measure, len(em.components))    // measures channel

	for _, id := range em.components {
		m := p.metrics[id]
		param := phono.Param{
			ID: id,
			Apply: func() error {
				var do struct{}
				mc <- m.Measure()
				done <- do
				return nil
			},
		}
		em.params = em.params.add(param)
	}

	//wait and close
	go func(requested int) {
		received := 0
		for received < requested {
			select {
			case <-done:
				received++
			case <-p.cancel:
				return
			}
		}
		close(mc)
	}(len(em.components))
	p.events <- em
	return mc
}

// start starts the execution of pipe.
func (p *Pipe) start() error {
	// build all runners first.
	// build pump.
	err := p.pump.build(p.ID())
	if err != nil {
		return err
	}

	// build processors.
	for _, proc := range p.processors {
		err := proc.build(p.ID())
		if err != nil {
			return err
		}
	}

	// build sinks.
	for _, sink := range p.sinks {
		err := sink.build(p.ID())
		if err != nil {
			return err
		}
	}

	errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
	// start pump
	out, errc := p.pump.run(p.cancel, p.ID(), p.provide, p.consume)
	if err != nil {
		interrupt(p.cancel)
		return err
	}
	errcList = append(errcList, errc)

	// start chained processesing
	for _, proc := range p.processors {
		out, errc = proc.run(p.cancel, p.ID(), out)
		if err != nil {
			interrupt(p.cancel)
			return err
		}
		errcList = append(errcList, errc)
	}

	sinkErrcList, err := p.broadcastToSinks(out)
	if err != nil {
		interrupt(p.cancel)
		return err
	}
	errcList = append(errcList, sinkErrcList...)
	p.errc = mergeErrors(errcList...)
	return nil
}

// broadcastToSinks passes messages to all sinks.
func (p *Pipe) broadcastToSinks(in <-chan message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan message)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc := s.run(p.cancel, p.ID(), broadcasts[i])
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

	return errcList, nil
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

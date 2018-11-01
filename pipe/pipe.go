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

// NewMessageFunc is a message-producer function
// sourceID expected to be pump's id
type newMessageFunc func() chan message

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
	cancel     chan struct{}

	pump       *pumpRunner
	processors []*processRunner
	sinks      []*sinkRunner

	metrics  map[string]measurable // metrics holds all references to measurable components
	params   params                //cahced params
	feedback params                //cached feedback
	errc     chan error            // errors channel
	eventc   chan eventMessage     // event channel

	providerc chan struct{} // ask for new message request
	consumerc chan message  // emission of messages

	log log.Logger
}

// Option provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Option func(p *Pipe) phono.ParamFunc

// ErrInvalidState is returned if pipe method cannot be executed at this moment.
var ErrInvalidState = errors.New("invalid state")

// ErrComponentNoID is used to cause a panic when new component without ID is added to pipe.
var ErrComponentNoID = errors.New("component have no ID value")

// New creates a new pipe and applies provided options
// returned pipe is in ready state
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
		eventc:     make(chan eventMessage, 1),
		providerc:  make(chan struct{}),
		consumerc:  make(chan message),
	}
	for _, option := range options {
		option(p)()
	}
	go func() {
		var s State = Ready
		t := target{}
		for s != nil {
			s, t = s.listen(p, t)
			p.log.Debug(fmt.Sprintf("%v is %T", p, s))
		}
		// cancel last pending target
		t.dismiss()
	}()
	return p
}

// WithName sets name to Pipe
func WithName(n string) Option {
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.name = n
		}
	}
}

// WithPump sets pump to Pipe
func WithPump(pump phono.Pump) Option {
	if pump.ID() == "" {
		panic(ErrComponentNoID)
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			r := &pumpRunner{
				Pump:       pump,
				Flusher:    flusher(pump),
				measurable: newMetric(pump.ID(), p.sampleRate, counters.pump...),
			}
			p.metrics[r.ID()] = r
			p.pump = r
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
		return func() {
			for i := range processors {
				r := &processRunner{
					Processor:  processors[i],
					Flusher:    flusher(processors[i]),
					measurable: newMetric(processors[i].ID(), p.sampleRate, counters.processor...),
				}
				p.metrics[r.ID()] = r
				p.processors = append(p.processors, r)
			}
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
		return func() {
			for i := range sinks {
				r := &sinkRunner{
					Sink:       sinks[i],
					Flusher:    flusher(sinks[i]),
					measurable: newMetric(sinks[i].ID(), p.sampleRate, counters.sink...),
				}
				p.metrics[r.ID()] = r
				p.sinks = append(p.sinks, r)
			}
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
	p.eventc <- eventMessage{
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
// TODO: prevent leak.
func (p *Pipe) Measure(ids ...string) <-chan Measure {
	if len(p.metrics) == 0 {
		return nil
	}
	em := eventMessage{event: measure, params: make(map[string][]phono.ParamFunc)}
	if len(ids) > 0 {
		em.callbacks = make([]string, 0, len(ids))
		// check if passed ids are part of the pipe
		for _, id := range ids {
			_, ok := p.metrics[id]
			if ok {
				em.callbacks = append(em.callbacks, id)
			}
		}
		// no components found
		if len(em.callbacks) == 0 {
			return nil
		}
	} else {
		em.callbacks = make([]string, 0, len(p.metrics))
		// get all if nothin is passed
		for id := range p.metrics {
			em.callbacks = append(em.callbacks, id)
		}
	}
	// waitgroup to close metrics channel
	var wg sync.WaitGroup
	wg.Add(len(em.callbacks))
	// measures channel
	mc := make(chan Measure, len(em.callbacks))
	for _, id := range em.callbacks {
		m := p.metrics[id]
		param := phono.Param{
			ID: id,
			Apply: func() {
				mc <- m.Measure()
				wg.Done()
			},
		}
		em.params = em.params.add(param)
	}
	//wait and close
	go func() {
		wg.Wait()
		close(mc)
	}()
	p.eventc <- em
	return mc
}

// Close must be called to clean up pipe's resources.
func (p *Pipe) Close() {
	defer func() {
		recover()
	}()
	close(p.eventc)
}

// merge error channels.
// TODO: prevent leaking.
func mergeErrors(errcList ...<-chan error) chan error {
	var wg sync.WaitGroup
	out := make(chan error, len(errcList))

	//function to wait for error channel
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(errcList))
	for _, e := range errcList {
		go output(e)
	}

	//wait and close out
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// broadcastToSinks passes messages to all sinks.
// TODO: prevent leaking.
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
		errc := s.run(p.ID(), broadcasts[i])
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
				broadcasts[i] <- m
			}
		}
	}()

	return errcList, nil
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

// soure returns a default message producer which will be sent to pump.
func (p *Pipe) source() newMessageFunc {
	// if pipe paused this call will block
	return func() chan message {
		var do struct{}
		p.providerc <- do
		return p.consumerc
	}
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

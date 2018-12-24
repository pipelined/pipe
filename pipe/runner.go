package pipe

import (
	"time"

	"github.com/dudk/phono"
)

// pumpRunner is pump's runner.
type pumpRunner struct {
	sampleRate phono.SampleRate
	phono.Pump
	fn  phono.PumpFunc
	out chan message
	hooks
}

// processRunner represents processor's runner.
type processRunner struct {
	sampleRate phono.SampleRate
	phono.Processor
	fn  phono.ProcessFunc
	in  <-chan message
	out chan message
	hooks
}

// sinkRunner represents sink's runner.
type sinkRunner struct {
	sampleRate phono.SampleRate
	phono.Sink
	fn phono.SinkFunc
	in <-chan message
	hooks
}

// Flusher defines component that must flushed in the end of execution.
type Flusher interface {
	Flush(string) error
}

// Interrupter defines component that has custom interruption logic.
type Interrupter interface {
	Interrupt(string) error
}

// Resetter defines component that must be resetted before consequent use.
type Resetter interface {
	Reset(string) error
}

// hook represents optional functions for components lyfecycle.
type hook func(string) error

// set of hooks for runners.
type hooks struct {
	flush     hook
	interrupt hook
	reset     hook
}

// bindHooks of component.
func bindHooks(v interface{}) hooks {
	return hooks{
		flush:     flusher(v),
		interrupt: interrupter(v),
		reset:     resetter(v),
	}
}

type meter struct {
	phono.Meter
	sampleRate  phono.SampleRate
	startedAt   time.Time     // StartCounter
	messages    int64         // MessageCounter
	samples     int64         // SampleCounter
	latency     time.Duration // LatencyCounter
	processedAt time.Time
	elapsed     time.Duration // ElapsedCounter
	duration    time.Duration // DurationCounter
}

// newMeter creates new meter with counters.
func newMeter(componentID string, sampleRate phono.SampleRate, m phono.Metric) meter {
	meter := meter{
		sampleRate:  sampleRate,
		startedAt:   time.Now(),
		processedAt: time.Now(),
	}
	if m != nil {
		meter.Meter = m.Meter(componentID, counters...)
		meter.Store(StartCounter, meter.startedAt)
	}

	return meter
}

// message capture metrics after message is processed.
func (m meter) message() meter {
	m.messages++
	m.latency = time.Since(m.processedAt)
	m.processedAt = time.Now()
	m.elapsed = time.Since(m.startedAt)
	if m.Meter != nil {
		m.Store(MessageCounter, m.messages)
		m.Store(LatencyCounter, m.latency)
		m.Store(ElapsedCounter, m.elapsed)
	}
	return m
}

// sample capture metrics after samples are processed.
func (m meter) sample(s int64) meter {
	m.samples = m.samples + s
	m.duration = m.sampleRate.DurationOf(m.samples)
	if m.Meter != nil {
		m.Store(SampleCounter, m.samples)
		m.Store(DurationCounter, m.duration)
	}
	return m
}

// counters is a structure for metrics initialization.
var counters = []string{MessageCounter, SampleCounter, StartCounter, LatencyCounter, DurationCounter, ElapsedCounter}

var do struct{}

const (
	// MessageCounter measures number of messages.
	MessageCounter = "Messages"
	// SampleCounter measures number of samples.
	SampleCounter = "Samples"
	// StartCounter fixes when runner started.
	StartCounter = "Start"
	// LatencyCounter measures latency between processing calls.
	LatencyCounter = "Latency"
	// ElapsedCounter fixes when runner ended.
	ElapsedCounter = "Elapsed"
	// DurationCounter counts what's the duration of signal.
	DurationCounter = "Duration"
)

// flusher checks if interface implements Flusher and if so, return it.
func flusher(i interface{}) hook {
	if v, ok := i.(Flusher); ok {
		return v.Flush
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func interrupter(i interface{}) hook {
	if v, ok := i.(Interrupter); ok {
		return v.Interrupt
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func resetter(i interface{}) hook {
	if v, ok := i.(Resetter); ok {
		return v.Reset
	}
	return nil
}

// newPumpRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newPumpRunner(sampleRate phono.SampleRate, sourceID string, p phono.Pump) (*pumpRunner, error) {
	fn, err := p.Pump(sourceID)
	if err != nil {
		return nil, err
	}
	r := pumpRunner{
		sampleRate: sampleRate,
		fn:         fn,
		Pump:       p,
		hooks:      bindHooks(p),
	}
	return &r, nil
}

// run the Pump runner.
func (r *pumpRunner) run(cancel chan struct{}, sourceID string, provide chan struct{}, consume chan message, metric phono.Metric) (<-chan message, <-chan error) {
	out := make(chan message)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)
		call(r.reset, sourceID, errc) // reset hook
		var err error
		var m message
		meter := newMeter(r.ID(), r.sampleRate, metric)
		for {
			// request new message
			select {
			case provide <- do:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			// receive new message
			select {
			case m = <-consume:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.applyTo(r.ID())      // apply params
			m.Buffer, err = r.fn() // pump new buffer
			if err != nil {
				if err == phono.ErrEOP {
					call(r.flush, sourceID, errc) // flush hook
				} else {
					errc <- err
				}
				return
			}

			meter = meter.sample(int64(m.Buffer.Size())).message()

			m.feedback.applyTo(r.ID()) // apply feedback

			// push message further
			select {
			case out <- m:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}
		}
	}()
	return out, errc
}

// newProcessRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newProcessRunner(sampleRate phono.SampleRate, sourceID string, p phono.Processor) (*processRunner, error) {
	fn, err := p.Process(sourceID)
	if err != nil {
		return nil, err
	}
	r := processRunner{
		sampleRate: sampleRate,
		fn:         fn,
		Processor:  p,
		hooks:      bindHooks(p),
	}
	return &r, nil
}

// run the Processor runner.
func (r *processRunner) run(cancel chan struct{}, sourceID string, in <-chan message, metric phono.Metric) (<-chan message, <-chan error) {
	errc := make(chan error, 1)
	r.in = in
	r.out = make(chan message)
	go func() {
		defer close(r.out)
		defer close(errc)
		meter := newMeter(r.ID(), r.sampleRate, metric)
		call(r.reset, sourceID, errc) // reset hook
		var err error
		var m message
		var ok bool
		for {
			// retrieve new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, sourceID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.applyTo(r.Processor.ID())    // apply params
			m.Buffer, err = r.fn(m.Buffer) // process new buffer
			if err != nil {
				errc <- err
				return
			}

			meter = meter.sample(int64(m.Buffer.Size())).message()

			m.feedback.applyTo(r.ID()) // apply feedback

			// send message further
			select {
			case r.out <- m:
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}
		}
	}()
	return r.out, errc
}

// newSinkRunner creates the closure. it's separated from run to have pre-run
// logic executed in correct order for all components.
func newSinkRunner(sampleRate phono.SampleRate, sourceID string, s phono.Sink) (*sinkRunner, error) {
	fn, err := s.Sink(sourceID)
	if err != nil {
		return nil, err
	}
	r := sinkRunner{
		sampleRate: sampleRate,
		fn:         fn,
		Sink:       s,
		hooks:      bindHooks(s),
	}
	return &r, nil
}

// run the sink runner.
func (r *sinkRunner) run(cancel chan struct{}, sourceID string, in <-chan message, metric phono.Metric) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		meter := newMeter(r.ID(), r.sampleRate, metric)
		call(r.reset, sourceID, errc) // reset hook
		var m message
		var ok bool
		for {
			// receive new message
			select {
			case m, ok = <-in:
				if !ok {
					call(r.flush, sourceID, errc) // flush hook
					return
				}
			case <-cancel:
				call(r.interrupt, sourceID, errc) // interrupt hook
				return
			}

			m.params.applyTo(r.Sink.ID()) // apply params
			err := r.fn(m.Buffer)         // sink a buffer
			if err != nil {
				errc <- err
				return
			}

			meter = meter.sample(int64(m.Buffer.Size())).message()

			m.feedback.applyTo(r.ID()) // apply feedback
		}
	}()

	return errc
}

// call optional function with sourceID argument. if error happens, it will be send to errc.
func call(fn hook, sourceID string, errc chan error) {
	if fn == nil {
		return
	}
	if err := fn(sourceID); err != nil {
		errc <- err
	}
	return
}

package pipe

import (
	"context"
	"errors"
	"sync"

	"github.com/dudk/phono"
)

// PumpRunner represents pump's runner.
type PumpRunner struct {
	phono.Pump
	Measurable
	Flusher
	out chan *message
}

// ProcessRunner represents processor's runner.
type ProcessRunner struct {
	phono.Processor
	Measurable
	Flusher
	in  <-chan *message
	out chan *message
}

// SinkRunner represents sink's runner.
type SinkRunner struct {
	phono.Sink
	Measurable
	Flusher
	in <-chan *message
}

// Flusher owns resource that has to be flushed in the end of execution.
type Flusher interface {
	Flush(string) error
}

// FlushFunc represents clean up function which is executed after loop is finished.
type FlushFunc func(string) error

var (
	// ErrSingleUseReused is returned when object designed for single-use is being reused.
	ErrSingleUseReused = errors.New("Error reuse single-use object")
)

const (
	// OutputCounter is a key for output counter within metric.
	// It calculates regular total output per component.
	OutputCounter = "Output"
)

func NewPump(p phono.Pump, m Measurable) *PumpRunner {
	r := &PumpRunner{
		Pump:       p,
		Measurable: m,
	}
	r.Measurable.AddCounters(OutputCounter)
	if v, ok := p.(Flusher); ok {
		r.Flusher = v
	}
	return r
}

func NewProcessor(p phono.Processor, m Measurable) *ProcessRunner {
	r := &ProcessRunner{
		Processor:  p,
		Measurable: m,
	}
	r.Measurable.AddCounters(OutputCounter)
	if v, ok := p.(Flusher); ok {
		r.Flusher = v
	}
	return r
}

func NewSink(s phono.Sink, m Measurable) *SinkRunner {
	r := &SinkRunner{
		Sink:       s,
		Measurable: m,
	}
	r.Measurable.AddCounters(OutputCounter)
	if v, ok := s.(Flusher); ok {
		r.Flusher = v
	}
	return r
}

// Run the Pump runner
func (p *PumpRunner) Run(ctx context.Context, sourceID string, newMessage newMessageFunc) (<-chan *message, <-chan error, error) {
	p.Measurable.Reset()
	pumpFn, err := p.Pump.Pump(sourceID)
	if err != nil {
		return nil, nil, err
	}
	out := make(chan *message)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		defer func() {
			if p.Flusher != nil {
				err := p.Flusher.Flush(sourceID)
				if err != nil {
					errc <- err
				}
			}
		}()
		defer p.FinishMeasure()
		p.Latency()
		var err error
		for {
			m := newMessage()
			m.applyTo(p.Pump.ID())
			m.Buffer, err = pumpFn()
			if err != nil {
				if err != ErrEOP {
					errc <- err
				}
				return
			}
			p.Counter(OutputCounter).Advance(m.Buffer)
			select {
			case <-ctx.Done():
				return
			default:
				out <- m
			}
			p.Latency()
		}
	}()
	return out, errc, nil
}

// SetMetric assigns new metrics and sets up the counters
func (p *PumpRunner) SetMetric(m Measurable) {
	p.Measurable = m
	p.Measurable.AddCounters(OutputCounter)
}

// Run the Processor runner
func (p *ProcessRunner) Run(sourceID string, in <-chan *message) (<-chan *message, <-chan error, error) {
	p.Measurable.Reset()
	// err := p.Before.call()
	processFn, err := p.Process(sourceID)
	if err != nil {
		return nil, nil, err
	}
	errc := make(chan error, 1)
	p.in = in
	p.out = make(chan *message)
	go func() {
		defer close(p.out)
		defer close(errc)
		defer func() {
			if p.Flusher != nil {
				err := p.Flusher.Flush(sourceID)
				if err != nil {
					errc <- err
				}
			}
		}()
		defer p.FinishMeasure()
		p.Latency()
		var err error
		for in != nil {
			select {
			case m, ok := <-in:
				if !ok {
					return
				}
				m.applyTo(p.Processor.ID())
				m.Buffer, err = processFn(m.Buffer)
				if err != nil {
					errc <- err
					return
				}
				p.Counter(OutputCounter).Advance(m.Buffer)
				p.out <- m
			}
			p.Latency()
		}
	}()
	return p.out, errc, nil
}

// SetMetric assigns new metrics and sets up the counters
func (p *ProcessRunner) SetMetric(m Measurable) {
	p.Measurable = m
	p.Measurable.AddCounters(OutputCounter)
}

// Run the sink runner
func (s *SinkRunner) Run(sourceID string, in <-chan *message) (<-chan error, error) {
	s.Measurable.Reset()
	// err := s.Before.call()
	sinkFn, err := s.Sink.Sink(sourceID)
	if err != nil {
		return nil, err
	}
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		defer func() {
			if s.Flusher != nil {
				err := s.Flusher.Flush(sourceID)
				if err != nil {
					errc <- err
				}
			}
		}()
		defer s.FinishMeasure()
		s.Latency()
		for in != nil {
			select {
			case m, ok := <-in:
				if !ok {
					return
				}
				m.params.applyTo(s.Sink.ID())
				err = sinkFn(m.Buffer)
				if err != nil {
					errc <- err
					return
				}
				s.Counter(OutputCounter).Advance(m.Buffer)
			}
			s.Latency()
		}
	}()

	return errc, nil
}

// SetMetric assigns new metrics and sets up the counters
func (s *SinkRunner) SetMetric(m Measurable) {
	s.Measurable = m
	s.Measurable.AddCounters(OutputCounter)
}

// SingleUse is designed to be used in runner-return functions to define a single-use pipe elements
func SingleUse(once *sync.Once) (err error) {
	err = ErrSingleUseReused
	once.Do(func() {
		err = nil
	})
	return
}

// func (f Flusher) call(sourceID string) error {
// 	if fn == nil {
// 		return nil
// 	}
// 	return f.Flush(sourceID)
// }

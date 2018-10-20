package pipe

import (
	"context"

	"github.com/dudk/phono"
)

// pumpRunner is pump's runner.
type pumpRunner struct {
	phono.Pump
	measurable
	Flusher
	out chan *message
}

// processRunner represents processor's runner.
type processRunner struct {
	phono.Processor
	measurable
	Flusher
	in  <-chan *message
	out chan *message
}

// sinkRunner represents sink's runner.
type sinkRunner struct {
	phono.Sink
	measurable
	Flusher
	in <-chan *message
}

// Flusher owns resource that has to be flushed in the end of execution.
type Flusher interface {
	Flush(string) error
}

// FlushFunc represents clean up function which is executed after loop is finished.
type FlushFunc func(string) error

// counters is a structure for metrics initialization.
var counters = struct {
	pump      []string
	processor []string
	sink      []string
}{
	pump:      []string{OutputCounter},
	processor: []string{OutputCounter},
	sink:      []string{OutputCounter},
}

const (
	// OutputCounter is a key for output counter within metric.
	// It calculates regular total output per component.
	OutputCounter = "Output"
)

// flusher checks if interface implements Flusher and if so, return it.
func flusher(i interface{}) Flusher {
	if v, ok := i.(Flusher); ok {
		return v
	}
	return nil
}

// run the Pump runner.
func (p *pumpRunner) run(ctx context.Context, sourceID string, newMessage newMessageFunc) (<-chan *message, <-chan error, error) {
	p.measurable.Reset()
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
		defer p.measurable.FinishMeasure()
		p.measurable.Latency()
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

// run the Processor runner.
func (p *processRunner) run(sourceID string, in <-chan *message) (<-chan *message, <-chan error, error) {
	p.measurable.Reset()
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
		defer p.measurable.FinishMeasure()
		p.measurable.Latency()
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

// run the sink runner.
func (s *sinkRunner) run(sourceID string, in <-chan *message) (<-chan error, error) {
	s.measurable.Reset()
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
		defer s.measurable.FinishMeasure()
		s.measurable.Latency()
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

package mock

import (
	"os"
	"strconv"
	"time"

	"github.com/dudk/phono"
)

const (
	defaultBufferSize = 512
	defaultSampleRate = 44100

	// PumpMaxInterval is 2 seconds
	PumpMaxInterval = 2000
	// PumpDefaultLimit is 10 messages
	PumpDefaultLimit = 10
)

var (
	// SkipPortaudio detects that portaudio tests should be skipped.
	SkipPortaudio = false
)

func init() {
	SkipPortaudio, _ = strconv.ParseBool(os.Getenv("SKIP_PORTAUDIO"))
}

// Param types
type (
	// Limit messages
	Limit int
)

// Pump mocks a pipe.Pump interface.
type Pump struct {
	phono.UID
	Counter
	Interval time.Duration
	Limit
	Value float64
	phono.BufferSize
	phono.NumChannels
}

// IntervalParam pushes new interval value for pump.
func (p *Pump) IntervalParam(i time.Duration) phono.Param {
	return phono.Param{
		ID: p.ID(),
		Apply: func() {
			p.Interval = i
		},
	}
}

// LimitParam pushes new limit value for pump.
func (p *Pump) LimitParam(l Limit) phono.Param {
	return phono.Param{
		ID: p.ID(),
		Apply: func() {
			p.Limit = l
		},
	}
}

// ValueParam pushes new signal value for pump.
func (p *Pump) ValueParam(v float64) phono.Param {
	return phono.Param{
		ID: p.ID(),
		Apply: func() {
			p.Value = v
		},
	}
}

// NumChannelsParam pushes new number of channels for pump
func (p *Pump) NumChannelsParam(nc phono.NumChannels) phono.Param {
	return phono.Param{
		ID: p.ID(),
		Apply: func() {
			p.NumChannels = nc
		},
	}
}

// Pump returns new buffer for pipe.
func (p *Pump) Pump(string) (phono.PumpFunc, error) {
	p.Reset()
	return func() (phono.Buffer, error) {
		if Limit(p.Counter.Messages()) >= p.Limit {
			return nil, phono.ErrEOP
		}
		time.Sleep(p.Interval)

		b := phono.Buffer(make([][]float64, p.NumChannels))
		for i := range b {
			b[i] = make([]float64, p.BufferSize)
			for j := range b[i] {
				b[i][j] = p.Value
			}
		}
		p.Counter.Advance(b)
		return b, nil
	}, nil
}

// Processor mocks a pipe.Processor interface.
type Processor struct {
	phono.UID
	Counter
}

// Process implementation for runner
func (p *Processor) Process(string) (phono.ProcessFunc, error) {
	p.Reset()
	return func(b phono.Buffer) (phono.Buffer, error) {
		p.Counter.Advance(b)
		return b, nil
	}, nil
}

// Sink mocks up a pipe.Sink interface.
// Buffer is not thread-safe, so should not be checked while pipe is running.
type Sink struct {
	phono.UID
	Counter
	phono.Buffer
}

// Sink implementation for runner.
func (s *Sink) Sink(string) (phono.SinkFunc, error) {
	s.Buffer = nil
	s.Reset()
	return func(b phono.Buffer) error {
		s.Buffer = s.Buffer.Append(b)
		s.Counter.Advance(b)
		return nil
	}, nil
}

// Counter counts messages and samples.
// Duration is not zero only in context of measure.
type Counter struct {
	messages int64
	samples  int64
}

// Advance counter's metrics.
func (c *Counter) Advance(buf phono.Buffer) {
	c.messages++
	c.samples = c.samples + int64(buf.Size())
}

// Reset resets counter's metrics.
func (c *Counter) Reset() {
	c.messages, c.samples = 0, 0
}

// Count returns messages and samples metrics.
func (c *Counter) Count() (int64, int64) {
	return c.messages, c.samples
}

// Messages returns messages metrics.
func (c *Counter) Messages() int64 {
	return c.messages
}

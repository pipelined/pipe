package mock

import (
	"os"
	"strconv"
	"time"

	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
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
	pipe.Counter
	Interval time.Duration
	Limit
	Value float64
	phono.BufferSize
	phono.NumChannels
	newMessage phono.NewMessageFunc
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
			return nil, pipe.ErrEOP
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
	pipe.Counter
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
	pipe.Counter
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

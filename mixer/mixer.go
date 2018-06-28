package mixer

import (
	"sync"

	"github.com/dudk/phono/pipe/runner"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
	"github.com/dudk/phono/pipe"
)

// Mixer summs up multiple channels of messages into a single channel
type Mixer struct {
	phono.UID
	log.Logger
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize

	// channel to send signals about closing inputs
	close chan string
	// channel to send frames ready for mix
	ready chan *frame

	sent int64

	sync.RWMutex
	inputs map[string]*Input
	done   map[string]*Input
	*frame
}

// Input represents a mixer input and is getting created everytime Sink method is called
type Input struct {
	// pos buffers
	*frame
	received int64
}

// message is used to pass received message from sink goroutines to pump
type message struct {
	message *phono.Message
	in      string
	close   bool
}

// frame represents a slice of samples to mix
type frame struct {
	sync.Mutex
	buffers  []phono.Buffer
	expected int
	next     *frame
}

func (f *frame) isFull() bool {
	return f.expected == len(f.buffers)
}

// sum returns a mixed samples
func (f *frame) sum(numChannels phono.NumChannels, bufferSize phono.BufferSize) phono.Buffer {
	var sum float64
	var signals float64
	result := phono.Buffer(make([][]float64, numChannels))
	for nc := 0; nc < int(numChannels); nc++ {
		result[nc] = make([]float64, 0, bufferSize)
		for bs := 0; bs < int(bufferSize); bs++ {
			sum = 0
			signals = 0
			// additional check to sum shorten blocks
			for i := 0; i < len(f.buffers) && len(f.buffers[i][nc]) > bs; i++ {
				sum = sum + f.buffers[i][nc][bs]
				signals++
			}
			result[nc] = append(result[nc], sum/signals)
		}
	}
	return result
}

const (
	defaultBufferLimit = 256
	enabled            = true
	disabled           = false
)

// New returns a new mixer
func New(bs phono.BufferSize, nc phono.NumChannels) *Mixer {
	m := &Mixer{
		Logger:      log.GetLogger(),
		close:       make(chan string, defaultBufferLimit),
		inputs:      make(map[string]*Input),
		done:        make(map[string]*Input),
		numChannels: nc,
		bufferSize:  bs,
	}
	return m
}

// RunPump returns initialized pump runner
func (m *Mixer) RunPump(sourceID string) pipe.PumpRunner {
	return &runner.Pump{
		Pump: m,
		Before: func() error {
			for sourceID, input := range m.done {
				input.frame = nil
				input.received = 0
				m.inputs[sourceID] = input
				delete(m.done, sourceID)
			}
			m.frame = &frame{}
			m.frame.expected = len(m.inputs)
			m.ready = make(chan *frame, defaultBufferLimit)
			return nil
		},
	}
}

// Pump returns a pump function which allows to read the out channel
func (m *Mixer) Pump(msg *phono.Message) (*phono.Message, error) {
	for {
		select {
		case inputID := <-m.close:
			m.Lock()
			input := m.inputs[inputID]
			for f := input.frame; f != nil; f = f.next {
				f.Lock()
				f.expected--
				ready := f.isFull() && f.expected > 0
				f.Unlock()
				if ready {
					m.ready <- f
				}
			}
			m.done[inputID] = input
			delete(m.inputs, inputID)
			if len(m.inputs) == 0 {
				close(m.ready)
			}
			m.Unlock()
		case f, ok := <-m.ready:
			if !ok {
				return nil, pipe.ErrEOP
			}
			m.Lock()
			m.frame = f.next
			m.Unlock()
			msg.Buffer = f.sum(m.numChannels, m.bufferSize)
			m.sent++
			return msg, nil
		}
	}
}

// RunSink returns initialized sink runner
func (m *Mixer) RunSink(sourceID string) pipe.SinkRunner {
	return &runner.Sink{
		Sink: m,
		Before: func() error {
			m.Lock()
			m.inputs[sourceID] = &Input{frame: m.frame}
			m.frame.expected = len(m.inputs)
			m.Unlock()
			return nil
		},
		After: func() error {
			m.close <- sourceID
			return nil
		},
	}
}

// Sink processes the single input
func (m *Mixer) Sink(msg *phono.Message) error {
	// get input info from mixer
	m.RLock()
	input := m.inputs[msg.SourceID]
	input.received++
	numInputs := len(m.inputs)
	// first sink call
	if input.frame == nil {
		input.frame = m.frame
	}
	m.RUnlock()

	input.frame.Lock()
	input.frame.buffers = append(input.frame.buffers, msg.Buffer)
	ready := input.frame.isFull()
	if input.frame.next == nil {
		input.frame.next = &frame{expected: numInputs}
	}
	input.frame.Unlock()

	if ready {
		m.ready <- input.frame
	}

	input.frame = input.frame.next
	return nil
}

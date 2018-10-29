package mixer

import (
	"sync"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
)

// Mixer summs up multiple channels of messages into a single channel.
type Mixer struct {
	phono.UID
	log.Logger
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize
	ready       chan *frame       // channel to send frames ready for mix
	in          chan *inMessage   // channel to send incoming messages
	inputs      map[string]*input // inputs sinking data
	done        map[string]*input // output for pumping data
	*frame

	sync.Mutex
	outputID string // id of the pipe which is output of mixer
}

type inMessage struct {
	sourceID string
	phono.Buffer
}

// input represents a mixer input and is getting created everytime Sink method is called.
type input struct {
	*frame
}

// frame represents a slice of samples to mix
type frame struct {
	buffers  []phono.Buffer
	expected int
	next     *frame
}

func (f *frame) isReady() bool {
	return f.expected == len(f.buffers)
}

// sum returns mixed samples.
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
	maxInputs = 1024
)

// New returns new mixer.
func New(bs phono.BufferSize, nc phono.NumChannels) *Mixer {
	m := &Mixer{
		UID:         phono.NewUID(),
		Logger:      log.GetLogger(),
		inputs:      make(map[string]*input),
		done:        make(map[string]*input),
		numChannels: nc,
		bufferSize:  bs,
		in:          make(chan *inMessage, maxInputs),
		frame:       &frame{},
	}
	return m
}

// Sink processes the single input.
func (m *Mixer) Sink(sourceID string) (phono.SinkFunc, error) {
	m.Lock()
	m.inputs[sourceID] = &input{frame: m.frame}
	m.frame.expected = len(m.inputs)
	m.Unlock()
	return func(b phono.Buffer) error {
		m.in <- &inMessage{sourceID: sourceID, Buffer: b}
		return nil
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	m.Lock()
	defer m.Unlock()
	if sourceID == m.outputID {
		m.frame = &frame{}
		return nil
	}
	m.in <- &inMessage{sourceID: sourceID}
	return nil
}

// Pump returns a pump function which allows to read the out channel.
func (m *Mixer) Pump(sourceID string) (phono.PumpFunc, error) {
	m.Lock()
	m.outputID = sourceID
	m.Unlock()

	// new out channel
	m.ready = make(chan *frame, 1)

	// reset old inputs
	for sourceID := range m.done {
		delete(m.done, sourceID)
	}

	// this goroutine lives while pump works.
	// TODO: prevent leaking.
	go func() {
		for msg := range m.in {
			m.Lock()
			input := m.inputs[msg.sourceID]
			m.Unlock()
			// buffer is nil only when input is closed
			if msg.Buffer != nil {
				input.frame.buffers = append(input.frame.buffers, msg.Buffer)
				if input.frame.next == nil {
					input.frame.next = &frame{expected: len(m.inputs)}
				}

				if input.frame.isReady() {
					m.sendFrame(input.frame)
				}
				// proceed input to next frame
				input.frame = input.frame.next
			} else {
				// lower expectations for each next frame
				for f := input.frame; f != nil; f = f.next {
					f.expected--
					if f.expected > 0 && f.isReady() {
						m.sendFrame(input.frame)
					}
				}
				m.done[msg.sourceID] = input
				m.Lock()
				delete(m.inputs, msg.sourceID)
				m.Unlock()
				if len(m.inputs) == 0 {
					close(m.ready)
					return
				}
			}
		}
	}()
	return func() (phono.Buffer, error) {
		// receive new buffer
		f, ok := <-m.ready
		if !ok {
			return nil, phono.ErrEOP
		}
		b := f.sum(m.numChannels, m.bufferSize)
		return b, nil
	}, nil
}

func (m *Mixer) sendFrame(f *frame) {
	m.frame = f
	m.ready <- f
}

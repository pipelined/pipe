package mixer

import (
	"sync/atomic"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
)

// Mixer summs up multiple channels of messages into a single channel.
type Mixer struct {
	phono.UID
	log.Logger
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize
	out         chan *frame       // channel to send frames ready for mix
	in          chan *inMessage   // channel to send incoming messages
	inputs      map[string]*input // inputs sinking data
	done        []string          // done inputs ids
	register    chan string       // register new input input, buffered
	outputID    atomic.Value      // id of the pipe which is output of mixer
	*frame                        // last processed frame
}

type inMessage struct {
	sourceID string
	phono.Buffer
}

// input represents a mixer input and is getting created everytime Sink method is called.
type input struct {
	*frame
}

// frame represents a slice of samples to mix.
type frame struct {
	buffers  []phono.Buffer
	expected int
	next     *frame
}

// sum returns mixed samples.
func (f *frame) sum(numChannels phono.NumChannels, bufferSize phono.BufferSize) phono.Buffer {
	var sum float64
	var inputs float64
	result := phono.Buffer(make([][]float64, numChannels))
	for nc := 0; nc < int(numChannels); nc++ {
		result[nc] = make([]float64, 0, bufferSize)
		for bs := 0; bs < int(bufferSize); bs++ {
			sum = 0
			inputs = 0
			// additional check to sum shorten blocks.
			for i := 0; i < len(f.buffers) && len(f.buffers[i][nc]) > bs; i++ {
				sum = sum + f.buffers[i][nc][bs]
				inputs++
			}
			result[nc] = append(result[nc], sum/inputs)
		}
	}
	f.buffers = nil
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
		numChannels: nc,
		bufferSize:  bs,
		in:          make(chan *inMessage, 1),
		register:    make(chan string, maxInputs),
	}
	return m
}

// Sink registers new input.
func (m *Mixer) Sink(inputID string) (phono.SinkFunc, error) {
	m.register <- inputID
	return func(b phono.Buffer) error {
		m.in <- &inMessage{sourceID: inputID, Buffer: b}
		return nil
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	if m.isOutput(sourceID) {
		return nil
	}
	m.in <- &inMessage{sourceID: sourceID}
	return nil
}

// Reset resets the mixer for another run.
func (m *Mixer) Reset(sourceID string) error {
	if m.isOutput(sourceID) {
		m.out = make(chan *frame, 1)
		go m.mix()
	}
	return nil
}

func (m *Mixer) isOutput(sourceID string) bool {
	return sourceID == m.outputID.Load().(string)
}

// Pump returns a pump function which allows to read the out channel.
func (m *Mixer) Pump(outputID string) (phono.PumpFunc, error) {
	m.outputID.Store(outputID)
	return func() (phono.Buffer, error) {
		// receive new buffer
		f, ok := <-m.out
		if !ok {
			return nil, phono.ErrEOP
		}
		return f.sum(m.numChannels, m.bufferSize), nil
	}, nil
}

func (m *Mixer) mix() {
	m.frame = &frame{}
	for {
		select {
		// add all scheduled inputs.
		case s := <-m.register:
			m.inputs[s] = &input{m.frame}
		default:
			// add done inputs.
			for _, inputID := range m.done {
				m.inputs[inputID] = &input{m.frame}
			}
			m.done = make([]string, 0, len(m.inputs))

			// now we have all inputs, can initiate first frame.
			m.frame.expected = len(m.inputs)

			// start main loop.
			for {
				select {
				// register new input.
				case s := <-m.register:
					m.inputs[s] = &input{frame: m.frame}
				case msg := <-m.in:
					s := m.inputs[msg.sourceID]
					if msg.Buffer != nil {
						s.frame.buffers = append(s.frame.buffers, msg.Buffer)
						if s.frame.isReady() {
							m.send(s.frame)
						}

						// check if there is no next frame.
						if s.frame.next == nil {
							s.frame.next = newFrame(len(m.inputs))
						}
						// proceed input to next frame.
						s.frame = s.frame.next
					} else {
						m.done = append(m.done, msg.sourceID)
						delete(m.inputs, msg.sourceID)
						// lower expectations for each next frame.
						for f := s.frame; f != nil; f = f.next {
							f.expected = len(m.inputs)
							if f.expected > 0 && f.isReady() {
								m.send(s.frame)
							}
						}
						s.frame = nil
						if len(m.inputs) == 0 {
							close(m.out)
							return
						}
					}
				}
			}
		}
	}
}

// isReady checks if frame is completed.
func (f *frame) isReady() bool {
	return f.expected == len(f.buffers)
}

// newFrame generates new frame based on number of inputs.
func newFrame(numInputs int) *frame {
	return &frame{expected: numInputs}
}

func (m *Mixer) send(f *frame) {
	m.frame = f
	m.out <- f
}

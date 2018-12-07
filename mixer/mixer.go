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
	out         chan phono.Buffer // channel to send frames ready for mix
	in          chan *inMessage   // channel to send incoming messages
	inputs      map[string]*input // inputs sinking data
	done        map[string]*input // done inputs
	*frame                        // last processed frame

	outputID atomic.Value // id of the pipe which is output of mixer

	register      chan string // register new input input, buffered
	sinkcalls     int32       // incremented before send register
	sinkcallsdone int32       // incremented after send register
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
			// additional check to sum shorten blocks
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
		done:        make(map[string]*input),
		numChannels: nc,
		bufferSize:  bs,
		in:          make(chan *inMessage, 1),

		register: make(chan string, maxInputs),
	}
	return m
}

// Sink registers new input.
func (m *Mixer) Sink(inputID string) (phono.SinkFunc, error) {
	atomic.AddInt32(&m.sinkcalls, 1)
	m.register <- inputID
	atomic.AddInt32(&m.sinkcallsdone, 1)
	return func(b phono.Buffer) error {
		m.in <- &inMessage{sourceID: inputID, Buffer: b}
		return nil
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	if m.isPump(sourceID) {
		return nil
	}
	m.in <- &inMessage{sourceID: sourceID}
	return nil
}

// Reset resets the mixer for another run.
func (m *Mixer) Reset(sourceID string) error {
	if m.isPump(sourceID) {
		m.out = make(chan phono.Buffer, 1)
		go m.mix()
	}
	return nil
}

func (m *Mixer) isPump(sourceID string) bool {
	return sourceID == m.outputID.Load().(string)
}

// Pump returns a pump function which allows to read the out channel.
func (m *Mixer) Pump(sourceID string) (phono.PumpFunc, error) {
	m.outputID.Store(sourceID)
	return func() (phono.Buffer, error) {
		// receive new buffer
		b, ok := <-m.out
		if !ok {
			return nil, phono.ErrEOP
		}
		// b := f.sum(m.numChannels, m.bufferSize)
		return b, nil
	}, nil
}

func (m *Mixer) mix() {
	// first, add all scheduled inputs
	for {
		select {
		case s := <-m.register:
			m.inputs[s] = &input{}
		default:
			// add done inputs
			for k, v := range m.done {
				m.inputs[k] = v
				delete(m.done, k)
			}

			// now we have all inputs, can initiate frames
			m.frame = m.newFrame()
			for k := range m.inputs {
				m.inputs[k].frame = m.frame
			}

			// start main loop
			for {
				select {
				// register new input
				case s := <-m.register:
					m.inputs[s] = &input{}
					m.inputs[s].frame = m.frame
				case msg := <-m.in:
					s := m.inputs[msg.sourceID]
					if msg.Buffer != nil {
						s.frame.buffers = append(s.frame.buffers, msg.Buffer)
						if s.frame.isReady() {
							m.send(s.frame)
						}

						// check if there is no next frame
						if s.frame.next == nil {
							s.frame.next = m.newFrame()
						}
						// proceed input to next frame
						s.frame = s.frame.next
					} else {
						// lower expectations for each next frame
						for f := s.frame; f != nil; f = f.next {
							f.expected--
							if f.expected > 0 && f.isReady() {
								m.send(s.frame)
							}
						}
						m.done[msg.sourceID] = s
						delete(m.inputs, msg.sourceID)
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

func (f *frame) isReady() bool {
	// fmt.Printf("Is ready: %v expected: %v got: %v", f, f.expected, len(f.buffers))
	return f.expected == len(f.buffers)
}

func (m *Mixer) newFrame() *frame {
	return &frame{expected: len(m.inputs)}
}

func (m *Mixer) send(f *frame) {
	m.frame = f
	m.out <- f.sum(m.numChannels, m.bufferSize)
}

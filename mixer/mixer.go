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
	frames      map[string]*frame // frames sinking data
	done        []string          // done inputs ids
	register    chan string       // register new input input, buffered
	outputID    atomic.Value      // id of the pipe which is output of mixer
	*frame                        // last processed frame
}

type inMessage struct {
	inputID string
	phono.Buffer
}

// frame represents a slice of samples to mix.
type frame struct {
	buffers  []phono.Buffer
	expected int
	next     *frame
}

// sum returns mixed samplein.
func (f *frame) sum(numChannels phono.NumChannels, bufferSize phono.BufferSize) phono.Buffer {
	var sum float64
	var frames float64
	result := phono.Buffer(make([][]float64, numChannels))
	for nc := 0; nc < int(numChannels); nc++ {
		result[nc] = make([]float64, 0, bufferSize)
		for bs := 0; bs < int(bufferSize); bs++ {
			sum = 0
			frames = 0
			// additional check to sum shorten blockin.
			for i := 0; i < len(f.buffers) && len(f.buffers[i][nc]) > bs; i++ {
				sum = sum + f.buffers[i][nc][bs]
				frames++
			}
			result[nc] = append(result[nc], sum/frames)
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
		frames:      make(map[string]*frame),
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
		m.in <- &inMessage{inputID: inputID, Buffer: b}
		return nil
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	if m.isOutput(sourceID) {
		return nil
	}
	m.in <- &inMessage{inputID: sourceID}
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
		// add all scheduled inputin.
		case s := <-m.register:
			m.frames[s] = m.frame
		default:
			// add done inputin.
			for _, inputID := range m.done {
				m.frames[inputID] = m.frame
			}
			m.done = make([]string, 0, len(m.frames))

			// now we have all frames, can initiate first frame.
			m.frame.expected = len(m.frames)

			// start main loop.
			for {
				select {
				// register new input.
				case s := <-m.register:
					// add new input.
					m.frames[s] = m.frame
					resetExpectations(m.frame, len(m.frames))
				case msg := <-m.in:
					f := m.frames[msg.inputID]
					if msg.Buffer != nil {
						f.buffers = append(f.buffers, msg.Buffer)
						m.frame = send(f, m.out)

						// check if there is no next frame.
						if f.next == nil {
							f.next = newFrame(len(m.frames))
						}
						// proceed input to next frame.
						m.frames[msg.inputID] = f.next
					} else {
						// move input to done.
						m.done = append(m.done, msg.inputID)
						delete(m.frames, msg.inputID)
						// reset expectations.
						resetExpectations(f, len(m.frames))
						// send frames.
						m.frame = send(f, m.out)
						if len(m.frames) == 0 {
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
func (f *frame) isComplete() bool {
	return f.expected > 0 && f.expected == len(f.buffers)
}

// newFrame generates new frame based on number of inputin.
func newFrame(numframes int) *frame {
	return &frame{expected: numframes}
}

// send all complete frames. Returns next incomplete frame.
func send(f *frame, out chan *frame) *frame {
	for f.isComplete() {
		out <- f
		f = f.next
	}
	return f
}

// resetExpectations for all frames after this one.
func resetExpectations(f *frame, e int) {
	for f != nil {
		f.expected = e
		f = f.next
	}
}

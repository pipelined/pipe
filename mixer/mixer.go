package mixer

import (
	"fmt"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
	"github.com/dudk/phono/pipe"
)

// Mixer summs up multiple channels of messages into a single channel.
type Mixer struct {
	phono.UID
	log.Logger
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize
	open        chan string     // channel to send signals about new inputs
	ready       chan *frame     // channel to send frames ready for mix
	in          chan *inMessage // channel to send incoming messages
	outputID    string          // id of the pipe which is output of mixer
	inputs      map[string]*input
	done        map[string]*input
	*frame
}

type inMessage struct {
	sourceID string
	phono.Buffer
}

// input represents a mixer input and is getting created everytime Sink method is called.
type input struct {
	*frame
	received int64
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
	defaultBufferLimit = 256
	maxInputs          = 1024
)

// New returns new mixer.
func New(bs phono.BufferSize, nc phono.NumChannels) *Mixer {
	m := &Mixer{
		Logger:      log.GetLogger(),
		inputs:      make(map[string]*input),
		done:        make(map[string]*input),
		numChannels: nc,
		bufferSize:  bs,
		open:        make(chan string, maxInputs),
		in:          make(chan *inMessage, defaultBufferLimit),
	}
	return m
}

// Sink processes the single input.
func (m *Mixer) Sink(sourceID string) (phono.SinkFunc, error) {
	m.open <- sourceID
	return func(b phono.Buffer) error {
		m.in <- &inMessage{sourceID: sourceID, Buffer: b}
		return nil
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	if sourceID == m.outputID {
		return nil
	}
	m.in <- &inMessage{sourceID: sourceID}
	return nil
}

// Pump returns a pump function which allows to read the out channel.
func (m *Mixer) Pump(sourceID string) (phono.PumpFunc, error) {
	m.outputID = sourceID
	// new frame
	m.frame = &frame{}

	// new out channel
	m.ready = make(chan *frame, defaultBufferLimit)

	// reset old inputs
	for sourceID := range m.done {
		delete(m.done, sourceID)
		m.open <- sourceID
	}

	// this goroutine lives while pump works
	go func() {
		for {
			select {
			// first add all new inputs
			case sourceID := <-m.open:
				fmt.Println("NEW INPUT for ", sourceID)
				m.inputs[sourceID] = &input{frame: m.frame}
				m.frame.expected = len(m.inputs)
				fmt.Printf("INPUTS: %+v\n", m.inputs)
			// now start processing
			default:
				for msg := range m.in {
					// buffer is nil only when input is closed
					if msg.Buffer != nil {
						input := m.inputs[msg.sourceID]
						input.received++
						// first sink call
						// get frame from mixer
						if input.frame == nil {
							input.frame = m.frame
						}

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
						input := m.inputs[msg.sourceID]
						fmt.Printf("sourceID %v input %v\n", msg.sourceID, input)
						// lower expectations for each next frame
						for f := input.frame; f != nil; f = f.next {
							f.expected--
							if f.expected > 0 && f.isReady() {
								m.sendFrame(input.frame)
							}
						}
						m.done[msg.sourceID] = input
						delete(m.inputs, msg.sourceID)
						if len(m.inputs) == 0 {
							close(m.ready)
							return
						}
					}
				}
			}
		}
	}()
	return func() (phono.Buffer, error) {
		// receive new buffer
		f, ok := <-m.ready
		if !ok {
			return nil, pipe.ErrEOP
		}
		b := f.sum(m.numChannels, m.bufferSize)
		return b, nil
	}, nil
}

func (m *Mixer) sendFrame(f *frame) {
	m.frame = f
	m.ready <- f
}

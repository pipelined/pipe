package mixer

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/pipelined/signal"
)

// Mixer summs up multiple channels of messages into a single channel.
type Mixer struct {
	out      chan *frame       // channel to send frames ready for mix
	in       chan *inMessage   // channel to send incoming messages
	frames   map[string]*frame // frames sinking data
	done     []string          // done inputs ids
	register chan string       // register new input input, buffered
	outputID atomic.Value      // id of the pipe which is output of mixer
	*frame                     // last processed frame
	cancel   chan struct{}     // cancel is closed only when pump is interrupted

	m           sync.Mutex
	sampleRate  int
	numChannels int
}

type inMessage struct {
	inputID string
	buffer  signal.Float64
}

// frame represents a slice of samples to mix.
type frame struct {
	buffers  [][][]float64
	expected int
	next     *frame
}

// sum returns mixed samplein.
func (f *frame) sum(numChannels int, bufferSize int) [][]float64 {
	var sum float64
	var frames float64
	result := make([][]float64, numChannels)
	for nc := 0; nc < int(numChannels); nc++ {
		result[nc] = make([]float64, 0, bufferSize)
		for bs := 0; bs < bufferSize; bs++ {
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
func New() *Mixer {
	m := Mixer{
		frames:   make(map[string]*frame),
		in:       make(chan *inMessage, 1),
		register: make(chan string, maxInputs),
		cancel:   make(chan struct{}),
	}
	return &m
}

// Sink registers new input. SampleRate and NumChannels are propagated here, due to that Sink pipes should be called before Pump.
func (m *Mixer) Sink(inputID string, sampleRate, numChannel, bufferSize int) (func([][]float64) error, error) {
	m.m.Lock()
	m.sampleRate = sampleRate
	m.numChannels = numChannel
	m.m.Unlock()
	m.register <- inputID
	return func(b [][]float64) error {
		select {
		case m.in <- &inMessage{inputID: inputID, buffer: b}:
			return nil
		case <-m.cancel:
			return io.ErrClosedPipe
		}
	}, nil
}

// Flush mixer data for defined source.
func (m *Mixer) Flush(sourceID string) error {
	if m.isOutput(sourceID) {
		m.cancel = make(chan struct{})
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
func (m *Mixer) Pump(outputID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	m.m.Lock()
	sampleRate := m.sampleRate
	numChannels := m.numChannels
	m.m.Unlock()
	m.outputID.Store(outputID)
	return func() ([][]float64, error) {
		// receive new buffer
		f, ok := <-m.out
		if !ok {
			return nil, io.EOF
		}
		return f.sum(numChannels, bufferSize), nil
	}, sampleRate, numChannels, nil
}

// Interrupt impliments pipe.Interrupter.
func (m *Mixer) Interrupt(sourceID string) error {
	if !m.isOutput(sourceID) {
		m.in <- &inMessage{inputID: sourceID}
	} else {
		close(m.cancel)
	}
	return nil
}

func (m *Mixer) mix() {
	m.frame = &frame{}

	// regsiter available inputs.
register:
	for {
		select {
		// add all scheduled inputs.
		case s := <-m.register:
			m.frames[s] = m.frame
		default:
			// add done inputs.
			for _, inputID := range m.done {
				m.frames[inputID] = m.frame
			}
			m.done = make([]string, 0, len(m.frames))

			// now we have all frames, can initiate first frame.
			m.frame.expected = len(m.frames)
			break register
		}
	}

	defer close(m.out)
	// start main loop.
	for {
		select {
		case <-m.cancel:
			return
		// register new input during runtime.
		case s := <-m.register:
			// add new input.
			m.frames[s] = m.frame
			resetExpectations(m.frame, len(m.frames))
		case msg := <-m.in:
			f := m.frames[msg.inputID]
			if msg.buffer != nil {
				f.buffers = append(f.buffers, msg.buffer)
				m.frame = send(f, m.out, m.cancel)

				// proceed input to next frame.
				if f.next == nil {
					f.next = newFrame(len(m.frames))
				}
				m.frames[msg.inputID] = f.next
			} else {
				// move input to done.
				m.done = append(m.done, msg.inputID)
				delete(m.frames, msg.inputID)
				// reset expectations.
				resetExpectations(f, len(m.frames))
				// send frames.
				m.frame = send(f, m.out, m.cancel)
				if len(m.frames) == 0 {
					// close(m.out)
					return
				}
			}
		}
	}
}

// isReady checks if frame is completed.
func (f *frame) isComplete() bool {
	return f.expected > 0 && f.expected == len(f.buffers)
}

// newFrame generates new frame based on number of inputs.
func newFrame(numframes int) *frame {
	return &frame{expected: numframes}
}

// send all complete frames. Returns next incomplete frame.
func send(f *frame, out chan *frame, cancel chan struct{}) *frame {
	for f != nil && f.isComplete() {
		select {
		case out <- f:
			f = f.next
		case <-cancel:
			return nil
		}
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

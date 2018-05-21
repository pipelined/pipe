package mixer

import (
	"context"
	"sort"

	"github.com/dudk/phono"
)

// Mixer summs up multiple channels of messages into a single channel
type Mixer struct {
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize

	inputs map[bool]map[<-chan *phono.Message]*Input

	process chan *message

	buffer         map[uint64][]phono.Samples
	bufferCounters []uint64
	expectedInputs map[uint64]int
	counter        uint64
}

type message struct {
	message *phono.Message
	in      <-chan *phono.Message
	close   bool
}

// Input represents a mixer input and is getting created everytime Sink method is called
type Input struct {
	counter uint64
}

const (
	processBuffer = 256
	enabled       = true
	disabled      = false
)

// New returns a new mixer
func New() *Mixer {
	m := &Mixer{
		process:        make(chan *message, processBuffer),
		buffer:         make(map[uint64][]phono.Samples),
		expectedInputs: make(map[uint64]int),
		inputs:         make(map[bool]map[<-chan *phono.Message]*Input),
		bufferCounters: make([]uint64, 0),
	}
	m.inputs[enabled] = make(map[<-chan *phono.Message]*Input)
	m.inputs[disabled] = make(map[<-chan *phono.Message]*Input)
	return m
}

// Pump returns a pump function which allows to read the out channel
// TODO: only one pump goroutine is allowed
// TODO: add a slice of received blocks to check it when input becomes idle or closed
// LIMITATION: if one of the sources is significantly faster input is frequently enabled/disabled - performance can beaffected
func (m *Mixer) Pump() phono.PumpFunc {
	out := make(chan *phono.Message)
	m.counter = 0
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan *phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		go func() {
			defer close(out)
			defer close(m.process)
			defer close(errc)
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-m.process:
					if !ok {
						return
					}
					input := m.inputs[enabled][msg.in]
					// input's state didn't change
					if !msg.close {
						// if first sample for this buffer came in
						if _, exists := m.buffer[input.counter]; !exists {
							m.buffer[input.counter] = make([]phono.Samples, 0, len(m.inputs[enabled]))
							// set new expected inputs
							m.expectedInputs[input.counter] = len(m.inputs[enabled])
							// add new buffer counter
							m.bufferCounters = append(m.bufferCounters, input.counter)
						}
						// append new samples to buffer
						m.buffer[input.counter] = append(m.buffer[input.counter], msg.message.Samples)

						// check if all expected inputs received
						if m.expectedInputs[input.counter] == len(m.buffer[input.counter]) {
							// send message
							message := newMessage()
							message.Samples = m.sum(m.buffer[input.counter])
							out <- message

							// clean up
							delete(m.expectedInputs, input.counter)
							// clean up
							delete(m.buffer, input.counter)
							m.bufferCounters = m.bufferCounters[1:]
							m.counter++
						}

						input.counter++
					} else {
						// todo: handle input disable
						i := sort.Search(len(m.expectedInputs), func(i int) bool { return m.bufferCounters[i] >= input.counter })
						if i < len(m.expectedInputs) && m.bufferCounters[i] == input.counter {
							for ; i < len(m.bufferCounters); i++ {
								counter := m.bufferCounters[i]
								// oh crap...
								m.expectedInputs[counter]--

								// DUPLICATION START
								// check if all expected inputs received
								if m.expectedInputs[counter] == len(m.buffer[counter]) {
									// send message
									message := newMessage()
									message.Samples = m.sum(m.buffer[counter])
									out <- message

									// clean up
									delete(m.expectedInputs, counter)
									// clean up
									delete(m.buffer, counter)
									m.bufferCounters = m.bufferCounters[1:]
									m.counter++
								}
								// DUPLICATION DONE
							}
						} else {
							// x is not present in data,
							// but i is the index where it would be inserted.
						}
						m.inputs[disabled][msg.in] = m.inputs[enabled][msg.in]
						delete(m.inputs[enabled], msg.in)
					}
				}
			}
		}()
		return out, errc, nil
	}
}

func (m *Mixer) sum(in []phono.Samples) phono.Samples {
	result := phono.NewSamples(m.numChannels, m.bufferSize)
	for s := 0; s < int(m.bufferSize); s++ {
		for c := 0; c < int(m.numChannels); c++ {
			sum := float64(0)
			for _, samples := range in {
				sum = sum + samples[c][s]
			}
			result[c][s] = sum / float64(len(in))
		}
	}
	return result
}

// Sink adds a new input channel to mix
// this method should not be called after
func (m *Mixer) Sink() phono.SinkFunc {
	return func(ctx context.Context, in <-chan *phono.Message) (<-chan error, error) {
		// m.ins = append(m.ins, in)
		m.inputs[enabled][in] = &Input{}
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			for in != nil {
				select {
				case msg, ok := <-in:
					if !ok {
						// we need to pass this channel to process, all inputs managemend should be in pump
						m.process <- &message{close: true, in: in}
					} else {
						m.process <- &message{message: msg, in: in}
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return errc, nil
	}
}

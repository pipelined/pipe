package mixer

import (
	"context"
	"fmt"
	"sort"

	"github.com/dudk/phono"
)

// Mixer summs up multiple channels of messages into a single channel
type Mixer struct {
	numChannels phono.NumChannels
	bufferSize  phono.BufferSize

	inputs map[bool]map[<-chan *phono.Message]*Input

	process chan *message

	buffers        map[uint64]*buffer
	bufferCounters []uint64
	expectedInputs map[uint64]int
	counter        uint64
}

type message struct {
	message *phono.Message
	in      <-chan *phono.Message
	close   bool
}

// buffer represents a slice of samples to mix
type buffer struct {
	samples []phono.Samples
}

// sum returns a mixed samples
func (b *buffer) sum(numChannels phono.NumChannels, bufferSize phono.BufferSize) phono.Samples {
	var sum float64
	var signals float64
	result := phono.NewSamples(numChannels, bufferSize)
	for nc := 0; nc < int(numChannels); nc++ {
		for bs := 0; bs < int(bufferSize); bs++ {
			sum = 0
			signals = 0
			for i := 0; i < len(b.samples) && len(b.samples[i][nc]) > bs; i++ {
				sum = sum + b.samples[i][nc][bs]
				signals++
			}
			result[nc][bs] = sum / signals
		}
	}
	return result
}

// Input represents a mixer input and is getting created everytime Sink method is called
type Input struct {
	// counter of received buffers
	counter uint64
}

const (
	processBuffer = 256
	enabled       = true
	disabled      = false
)

// New returns a new mixer
func New(bs phono.BufferSize, nc phono.NumChannels) *Mixer {
	m := &Mixer{
		process:        make(chan *message, processBuffer),
		buffers:        make(map[uint64]*buffer),
		expectedInputs: make(map[uint64]int),
		inputs:         make(map[bool]map[<-chan *phono.Message]*Input),
		bufferCounters: make([]uint64, 0, processBuffer),
		numChannels:    nc,
		bufferSize:     bs,
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
				case msg := <-m.process:
					input := m.inputs[enabled][msg.in]
					// input's state didn't change
					if !msg.close {
						var b *buffer
						var exists bool
						enabledInputs := len(m.inputs[enabled])
						// if first sample for this buffer came in
						if b, exists = m.buffers[input.counter]; !exists {
							b = &buffer{samples: make([]phono.Samples, 0, enabledInputs)}
							m.buffers[input.counter] = b
							// set new expected inputs
							m.expectedInputs[input.counter] = enabledInputs
							// add new buffer counter
							m.bufferCounters = append(m.bufferCounters, input.counter)
						}
						// append new samples to buffer
						b.samples = append(b.samples, msg.message.Samples)

						// check if all expected inputs received
						if m.expectedInputs[input.counter] == len(b.samples) {
							// send message
							message := newMessage()
							message.Samples = b.sum(m.numChannels, m.bufferSize)
							out <- message

							// clean up
							delete(m.expectedInputs, input.counter)
							// clean up
							delete(m.buffers, input.counter)
							m.bufferCounters = m.bufferCounters[1:]
							m.counter++
						}
						input.counter++
					} else {
						// todo: handle input disable
						fmt.Printf("message: %v m.bufferCounters: %v m.expectedInputs: %v\n", msg, m.bufferCounters, m.expectedInputs)
						i := sort.Search(len(m.expectedInputs), func(i int) bool { return m.bufferCounters[i] >= input.counter })
						fmt.Printf("Found: %v\n", i)
						if i < len(m.expectedInputs) && m.bufferCounters[i] == input.counter {
							for i < len(m.bufferCounters) {
								counter := m.bufferCounters[i]
								fmt.Printf("Reducing expectations for %v\n", counter)
								m.expectedInputs[counter]--

								// DUPLICATION START
								// check if all expected inputs received
								b := m.buffers[counter]
								if m.expectedInputs[counter] == len(b.samples) {
									fmt.Printf("pushing message for %v\n", counter)
									// send message
									message := newMessage()
									message.Samples = b.sum(m.numChannels, m.bufferSize)
									out <- message

									// clean up
									delete(m.expectedInputs, counter)
									// clean up
									delete(m.buffers, counter)
									m.bufferCounters = m.bufferCounters[1:]
									m.counter++
								} else {
									// because slice was shorten, include index only here
									i++
								}
								// DUPLICATION DONE
							}
						}
						delete(m.inputs[enabled], msg.in)
					}
					if len(m.inputs[enabled]) == 0 && len(m.inputs[disabled]) == 0 {
						return
					}
				}
			}
		}()
		return out, errc, nil
	}
}

// Sink adds a new input channel to mix
// this method should not be called after
func (m *Mixer) Sink() phono.SinkFunc {
	return func(in <-chan *phono.Message) (<-chan error, error) {
		m.inputs[enabled][in] = &Input{}
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			for in != nil {
				select {
				case msg, ok := <-in:
					if !ok {
						m.process <- &message{close: true, in: in}
						return
					}
					m.process <- &message{message: msg, in: in}
				}
			}
		}()

		return errc, nil
	}
}

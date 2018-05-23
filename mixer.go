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

	process    chan *message
	buffers    map[index]*buffer
	indexOrder []index
	sent       index
}

// Input represents a mixer input and is getting created everytime Sink method is called
type Input struct {
	// pos buffers
	index
}

// message is used to pass received message from sink goroutines to pump
type message struct {
	message *phono.Message
	in      <-chan *phono.Message
	close   bool
}

// buffer represents a slice of samples to mix
type buffer struct {
	samples  []phono.Samples
	expected int
}

// index is a counter of recieved buffers
type index uint64

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

const (
	processBuffer = 256
	enabled       = true
	disabled      = false
)

// New returns a new mixer
func New(bs phono.BufferSize, nc phono.NumChannels) *Mixer {
	m := &Mixer{
		process:     make(chan *message, processBuffer),
		buffers:     make(map[index]*buffer),
		inputs:      make(map[bool]map[<-chan *phono.Message]*Input),
		indexOrder:  make([]index, 0, processBuffer),
		numChannels: nc,
		bufferSize:  bs,
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
	m.sent = 0
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
						if b, exists = m.buffers[input.index]; !exists {
							b = &buffer{
								samples:  make([]phono.Samples, 0, enabledInputs),
								expected: enabledInputs,
							}
							m.buffers[input.index] = b
							// add new buffer index
							m.indexOrder = append(m.indexOrder, input.index)
						}
						// append new samples to buffer
						b.samples = append(b.samples, msg.message.Samples)

						// check if all expected inputs received
						if b.expected == len(b.samples) {
							// send message
							message := newMessage()
							message.Samples = b.sum(m.numChannels, m.bufferSize)
							out <- message

							// clean up
							delete(m.buffers, input.index)
							m.indexOrder = m.indexOrder[1:]
							m.sent++
						}
						input.index++
					} else {
						i := sort.Search(len(m.indexOrder), func(i int) bool { return m.indexOrder[i] >= input.index })
						fmt.Printf("Found: %v\n", i)
						if i < len(m.indexOrder) && m.indexOrder[i] == input.index {
							for i < len(m.indexOrder) {
								index := m.indexOrder[i]
								fmt.Printf("Reducing expectations for %v\n", index)

								// DUPLICATION START
								// check if all expected inputs received
								b := m.buffers[index]
								b.expected--
								if b.expected == len(b.samples) {
									fmt.Printf("pushing message for %v\n", index)
									// send message
									message := newMessage()
									message.Samples = b.sum(m.numChannels, m.bufferSize)
									out <- message

									// clean up
									delete(m.buffers, index)
									m.indexOrder = m.indexOrder[1:]
									m.sent++
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

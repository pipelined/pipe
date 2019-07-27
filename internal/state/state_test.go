package state_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
)

const bufferSize = 1024

var testError = errors.New("Test error")

type startFuncMock struct {
	sendc chan string
}

// TODO: refactor mock for simplicity
func (m *startFuncMock) fn(send chan string, badCancel bool) state.StartFunc {
	m.sendc = send
	return func(bufferSize int, cancelc chan struct{}, givec chan<- string) []<-chan error {
		errc := make(chan error)
		go func() {
			for {
				select {
				case <-cancelc:
					if !badCancel {
						close(errc)
					} else {
						errc <- testError
					}
					return
				case s, ok := <-m.sendc:
					if ok {
						givec <- s
					} else {
						errc <- testError
					}
				}
			}
		}()
		return []<-chan error{errc}
	}
}

type newMessageFuncMock struct {
	sent int
}

func (m *newMessageFuncMock) fn() state.NewMessageFunc {
	return func(pipeID string) {
		m.sent++
	}
}

type pushParamsFuncMock struct {
	state.Params
}

func (m *pushParamsFuncMock) fn() state.PushParamsFunc {
	return func(params state.Params) {
		m.Params = m.Params.Append(params)
	}
}

func TestStates(t *testing.T) {
	cases := []struct {
		preparation       []state.Event
		events            []state.Event
		messages          int
		throwError        bool
		throwErrorOnClose bool
	}{
		{
			// Ready state
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Pause{Feedback: make(chan error)},
			},
		},
		{
			// Running state
			preparation: []state.Event{
				state.Run{Feedback: make(chan error)},
			},
			messages: 10,
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Feedback: make(chan error)},
			},
		},
		{
			// Running state
			preparation: []state.Event{
				state.Run{Feedback: make(chan error)},
			},
			throwError: true,
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Feedback: make(chan error)},
			},
		},
		{
			// Running state
			preparation: []state.Event{
				state.Run{Feedback: make(chan error)},
			},
			throwErrorOnClose: true,
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Feedback: make(chan error)},
			},
		},
		{
			// Paused state
			preparation: []state.Event{
				state.Run{Feedback: make(chan error)},
				state.Pause{Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Pause{Feedback: make(chan error)},
				state.Run{Feedback: make(chan error)},
			},
		},
		{
			// Running state after pause
			preparation: []state.Event{
				state.Run{Feedback: make(chan error)},
				state.Pause{Feedback: make(chan error)},
				state.Resume{Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Feedback: make(chan error)},
			},
		},
	}

	for _, c := range cases {
		var feedback chan error
		startMock := &startFuncMock{}
		newMessageMock := &newMessageFuncMock{}
		pushParamsMock := &pushParamsFuncMock{}
		p := &paramMock{uid: "params"}
		send := make(chan string)
		h := state.NewHandle(
			startMock.fn(send, c.throwErrorOnClose),
			newMessageMock.fn(),
			pushParamsMock.fn(),
		)
		go state.Loop(h, state.Ready)

		// reach tested state
		for _, e := range c.preparation {
			feedback = e.Errc()
			h.Eventc <- e
		}

		// push params
		h.Paramc <- p.params()

		// send messages/error
		if c.throwError {
			close(send)
			err := pipe.Wait(feedback)
			assert.Equal(t, testError, err)
			continue
		} else {
			for i := 0; i < c.messages; i++ {
				send <- "test"
			}
		}

		// test events
		for _, e := range c.events {
			h.Eventc <- e
			err := pipe.Wait(e.Errc())
			assert.Equal(t, state.ErrInvalidState, err)
		}
		errc := make(chan error)
		h.Eventc <- state.Close{Feedback: errc}
		err := pipe.Wait(errc)
		if c.throwErrorOnClose {
			assert.Equal(t, testError, err)
		} else {
			assert.Nil(t, err)
		}

		_, ok := pushParamsMock.Params[p.uid]
		assert.True(t, ok)

		assert.Equal(t, c.messages, newMessageMock.sent)
	}
}

package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
)

const bufferSize = 1024

var testError = errors.New("Test error")

type startFuncMock struct{}

// send channel is closed ONLY when any messages were sent
func (m *startFuncMock) fn(send chan struct{}, errorOnSend, errorOnClose error) state.StartFunc {
	return func(bufferSize int, cancelc <-chan struct{}, givec chan<- string) []<-chan error {
		errc := make(chan error)
		go func() {
			defer close(errc)
			// send messages
			for {
				select {
				case _, ok := <-send:
					if !ok {
						return
					}
					// send error if provided
					if errorOnSend != nil {
						errc <- errorOnSend
					} else {
						givec <- "test"
					}
				case <-cancelc: // block until cancelled
					// send error on close if provided
					if errorOnClose != nil {
						errc <- errorOnClose
					}
					return
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
	ctx, cancelFn := context.WithCancel(context.Background())
	cases := []struct {
		messages     int
		errorOnSend  error
		errorOnClose error
		preparation  []state.Event
		events       []state.Event
		cancel       context.CancelFunc
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
			messages: 10,
			preparation: []state.Event{
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
		},
		{
			// Running state
			errorOnSend: testError,
			preparation: []state.Event{
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
		},
		{
			// Running state
			errorOnClose: testError,
			preparation: []state.Event{
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
		},
		{
			// Paused state
			preparation: []state.Event{
				state.Run{Context: context.Background(), Feedback: make(chan error)},
				state.Pause{Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Pause{Feedback: make(chan error)},
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
		},
		{
			// Running state after pause
			preparation: []state.Event{
				state.Run{Context: context.Background(), Feedback: make(chan error)},
				state.Pause{Feedback: make(chan error)},
				state.Resume{Feedback: make(chan error)},
			},
			events: []state.Event{
				state.Resume{Feedback: make(chan error)},
				state.Run{Context: context.Background(), Feedback: make(chan error)},
			},
		},
		{
			// Running state and cancel context
			preparation: []state.Event{
				state.Run{Context: ctx, Feedback: make(chan error)},
			},
			cancel: cancelFn,
		},
	}

	for _, c := range cases {
		var (
			feedback chan error
		)
		messages := c.messages
		if c.errorOnSend != nil {
			messages = 1
		}
		startMock := &startFuncMock{}
		newMessageMock := &newMessageFuncMock{}
		pushParamsMock := &pushParamsFuncMock{}
		p := &paramMock{uid: "params"}
		send := make(chan struct{})
		h := state.NewHandle(
			startMock.fn(send, c.errorOnSend, c.errorOnClose),
			newMessageMock.fn(),
			pushParamsMock.fn(),
		)
		go state.Loop(h, state.Ready)

		// reach tested state
		// remember last feedback channel
		for _, e := range c.preparation {
			feedback = e.Errc()
			h.Eventc <- e
		}

		// push params
		h.Paramc <- p.params()

		// test events
		for _, e := range c.events {
			h.Eventc <- e
			err := pipe.Wait(e.Errc())
			assert.Equal(t, state.ErrInvalidState, err)
		}

		// send messages
		if messages > 0 {
			for i := 0; i < messages; i++ {
				send <- struct{}{}
			}
			close(send)
			err := pipe.Wait(feedback)
			assert.Equal(t, c.errorOnSend, err)
		}

		if c.cancel != nil {
			c.cancel()
		}

		// close
		errc := make(chan error)
		h.Eventc <- state.Close{Feedback: errc}
		err := pipe.Wait(errc)
		assert.Equal(t, c.errorOnClose, err)

		_, ok := pushParamsMock.Params[p.uid]
		assert.True(t, ok)
		assert.Equal(t, c.messages, newMessageMock.sent)
	}
	goleak.VerifyNoLeaks(t)
}

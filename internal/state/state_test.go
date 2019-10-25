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
		preparation  []transition
		events       []transition
		cancel       context.CancelFunc
	}{
		{
			// Ready state
			events: []transition{
				resume,
				pause,
			},
		},
		{
			// Running state
			messages: 10,
			preparation: []transition{
				run,
			},
			events: []transition{
				resume,
				run,
			},
		},
		{
			// Running state
			errorOnSend: testError,
			preparation: []transition{
				run,
			},
		},
		{
			// Running state
			errorOnClose: testError,
			preparation: []transition{
				run,
			},
			events: []transition{
				resume,
				run,
			},
		},
		{
			// Paused state
			preparation: []transition{
				run,
				pause,
			},
			events: []transition{
				pause,
				run,
			},
		},
		{
			// Running state after pause
			preparation: []transition{
				run,
				pause,
				resume,
			},
			events: []transition{
				resume,
				run,
			},
		},
		{
			// Running state and cancel context
			// message is needed to ensure params delivery
			messages: 1,
			preparation: []transition{
				runWithContext(ctx),
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
		for _, transition := range c.preparation {
			feedback = transition(h)
		}

		// push params
		h.Paramc <- p.params()

		// test events
		for _, transition := range c.events {
			err := pipe.Wait(transition(h))
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
		err := pipe.Wait(h.Stop())
		assert.Equal(t, c.errorOnClose, err)

		_, ok := pushParamsMock.Params[p.uid]
		assert.True(t, ok)
		assert.Equal(t, c.messages, newMessageMock.sent)
	}
	goleak.VerifyNoLeaks(t)
}

type transition func(*state.Handle) chan error

var (
	run = func(h *state.Handle) chan error {
		return h.Run(context.Background(), 0)
	}
	resume = func(h *state.Handle) chan error {
		return h.Resume()
	}
	pause = func(h *state.Handle) chan error {
		return h.Pause()
	}
)

func runWithContext(ctx context.Context) transition {
	return func(h *state.Handle) chan error {
		return h.Run(ctx, 0)
	}
}

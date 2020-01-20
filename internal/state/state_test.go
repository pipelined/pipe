package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/state"
)

var testError = errors.New("test error")

type startFuncMock struct{}

// send channel is closed ONLY when any messages were sent
func (m *startFuncMock) fn(send chan struct{}, errorOnSend, errorOnClose error) state.StartFunc {
	params := make(chan state.Params)
	return func(bufferSize int, cancel <-chan struct{}, puller chan<- chan state.Params) ([]<-chan error, error) {
		errs := make(chan error)
		go func() {
			defer close(errs)
			// send messages
			for {
				select {
				case _, ok := <-send:
					if !ok {
						return
					}
					// send error if provided
					if errorOnSend != nil {
						errs <- errorOnSend
					} else {
						puller <- params
						<-params
					}
				case <-cancel: // block until cancelled
					// send error on close if provided
					if errorOnClose != nil {
						errs <- errorOnClose
					}
					return
				}
			}
		}()
		return []<-chan error{errs}, nil
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
			errs chan error
		)
		messages := c.messages
		if c.errorOnSend != nil {
			messages = 1
		}
		startMock := &startFuncMock{}

		send := make(chan struct{})
		h := state.NewHandle(
			startMock.fn(send, c.errorOnSend, c.errorOnClose),
		)
		go state.Loop(h)

		// reach tested state
		// remember last errs channel
		for _, transition := range c.preparation {
			errs = transition(h)
		}

		// test events
		for _, transition := range c.events {
			err := pipe.Wait(transition(h))
			assert.Equal(t, state.ErrInvalidState, errors.Unwrap(err))
		}

		// send messages
		if messages > 0 {
			for i := 0; i < messages; i++ {
				send <- struct{}{}
			}
			close(send)
			err := pipe.Wait(errs)
			assert.Equal(t, c.errorOnSend, errors.Unwrap(err))
		}

		if c.cancel != nil {
			c.cancel()
		}

		// close
		err := pipe.Wait(h.Interrupt())
		assert.Equal(t, c.errorOnClose, errors.Unwrap(err))
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

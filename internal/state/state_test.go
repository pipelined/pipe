package state_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
)

const bufferSize = 1024

type startFuncMock struct {
	sync.Mutex
	givec chan<- string
}

func (m *startFuncMock) fn() state.StartFunc {
	return func(bufferSize int, cancelc chan struct{}, givec chan<- string) []<-chan error {
		m.givec = givec
		errc := make(chan error)
		go func() {
			<-cancelc
			close(errc)
		}()
		return []<-chan error{errc}
	}
}

func (m *startFuncMock) send() {
	m.givec <- "test"
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
		preparation []state.Event
		events      []state.Event
		messages    int
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
			messages: 1,
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
		startMock := &startFuncMock{}
		newMessageMock := &newMessageFuncMock{}
		pushParamsMock := &pushParamsFuncMock{}
		p := &paramMock{uid: "params"}
		h := state.NewHandle(
			startMock.fn(),
			newMessageMock.fn(),
			pushParamsMock.fn(),
		)
		go state.Loop(h, state.Ready)

		// reach tested state
		for _, e := range c.preparation {
			h.Eventc <- e
		}

		// push params
		h.Paramc <- p.params()

		// TODO: fix mock
		// for i := 0; i < c.messages; i++ {
		// 	fmt.Println("Send msg")
		// 	startMock.send()
		// }
		// assert.Equal(t, c.messages, newMessageMock.sent)

		// test events
		for _, e := range c.events {
			h.Eventc <- e
			err := pipe.Wait(e.Errc())
			assert.Equal(t, state.ErrInvalidState, err)
		}
		errc := make(chan error)
		h.Eventc <- state.Close{Feedback: errc}
		err := pipe.Wait(errc)
		assert.Nil(t, err)

		_, ok := pushParamsMock.Params[p.uid]
		assert.True(t, ok)
	}
}

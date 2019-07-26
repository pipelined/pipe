package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
)

const bufferSize = 1024

type startFuncMock struct{}

func (m startFuncMock) fn() state.StartFunc {
	return func(bufferSize int, cancelc chan struct{}, givec chan<- string) []<-chan error {
		errc := make(chan error)
		go func() {
			<-cancelc
			close(errc)
		}()
		return []<-chan error{errc}
	}
}

type newMessageFuncMock struct{}

func (m *newMessageFuncMock) fn() state.NewMessageFunc {
	return func(pipeID string) {}
}

type pushParamsFuncMock struct {
	state.Params
}

func (m *pushParamsFuncMock) fn() state.PushParamsFunc {
	return func(params state.Params) {
		m.Params = m.Params.Append(params)
	}
}

// Test Handle in the ready state.
func TestInvalidState(t *testing.T) {
	var (
		err            error
		errc           chan error
		ok             bool
		startMock      startFuncMock
		newMessageMock newMessageFuncMock
	)
	pushParamsMock := &pushParamsFuncMock{}
	readyParam := &paramMock{uid: "readyParam"}
	// runningParam := &paramMock{uid: "runningParam"}
	// pausedParam := &paramMock{uid: "pausedParam"}

	h := state.NewHandle(
		startMock.fn(),
		newMessageMock.fn(),
		pushParamsMock.fn(),
	)
	go state.Loop(h, state.Ready)

	_ = testReady(t, h, readyParam)
	// testRunning(t, h, runc, runningParam)
	// testPaused(t, h, pausedParam)

	errc = make(chan error)
	h.Eventc <- state.Close{Feedback: errc}
	err = pipe.Wait(errc)
	assert.Nil(t, err)

	_, ok = pushParamsMock.Params[readyParam.uid]
	assert.True(t, ok)
	// _, ok = pushParamsMock.Params[runningParam.uid]
	// assert.True(t, ok)
	// _, ok = pushParamsMock.Params[pausedParam.uid]
	// assert.True(t, ok)
}

func testReady(t *testing.T, h *state.Handle, m *paramMock) <-chan error {
	var (
		err  error
		errc chan error
	)
	// push params
	h.Paramc <- m.params()

	t.Logf("Test pause")
	// test invalid actions
	errc = make(chan error)
	h.Eventc <- state.Pause{Feedback: errc}
	err = pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	t.Logf("Test resume")
	errc = make(chan error)
	h.Eventc <- state.Resume{Feedback: errc}
	err = pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	// transition to the next state
	runc := make(chan error)
	h.Eventc <- state.Run{BufferSize: bufferSize, Feedback: runc}
	return runc
}

// func testRunning(t *testing.T, h *state.Handle, runc <-chan error, m *paramMock) {
// 	var err error
// 	// push params
// 	h.Push(m.uid, m.param())

// 	err = pipe.Wait(h.Resume())
// 	assert.Equal(t, state.ErrInvalidState, err)

// 	err = pipe.Wait(h.Run(bufferSize))
// 	assert.Equal(t, state.ErrInvalidState, err)

// 	// transition to the next state
// 	pausec := h.Pause()
// 	assert.NotNil(t, pausec)
// 	// this target has to be cancelled now
// 	assert.Nil(t, pipe.Wait(runc))
// }

// func testPaused(t *testing.T, h *state.Handle, m *paramMock) {
// 	var err error
// 	// push params
// 	h.Push(m.uid, m.param())

// 	err = pipe.Wait(h.Pause())
// 	assert.Equal(t, state.ErrInvalidState, err)

// 	err = pipe.Wait(h.Run(bufferSize))
// 	assert.Equal(t, state.ErrInvalidState, err)

// 	// transition to the next state
// 	resumec := h.Resume()
// 	assert.NotNil(t, resumec)
// }

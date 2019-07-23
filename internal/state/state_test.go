package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
)

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

func (m newMessageFuncMock) fn() state.NewMessageFunc {
	return func(pipeID string) {}
}

type pushParamsFuncMock struct{}

func (m pushParamsFuncMock) fn() state.PushParamsFunc {
	return func(componentID string, params state.Params) {}
}

// Test Handle in the ready state.
func TestReadyHandle(t *testing.T) {
	var err error
	h := newHandle()
	go state.Loop(h, state.Ready)
	err = pipe.Wait(h.Close())
	assert.Nil(t, err)
}

func newHandle() *state.Handle {
	var (
		startMock      startFuncMock
		newMessageMock newMessageFuncMock
		pushParamsMock pushParamsFuncMock
	)
	h := state.NewHandle(
		startMock.fn(),
		newMessageMock.fn(),
		pushParamsMock.fn(),
	)
	return h
}

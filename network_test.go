package pipe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/mock"
	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/internal/state"
	"go.uber.org/goleak"
)

const (
	bufferSize = 512
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSimple(t *testing.T) {
	n := newNet(t)
	runc := n.Run(bufferSize)
	assert.NotNil(t, runc)
	err := pipe.Wait(runc)
	assert.Nil(t, err)

	pipe.Wait(n.Close())
}

// Test Run method for all states.
func TestRun(t *testing.T) {
	// run while ready
	n := newNet(t)
	runc := n.Run(bufferSize)
	assert.NotNil(t, runc)
	err := pipe.Wait(runc)
	assert.Nil(t, err)

	// run while running
	runc = n.Run(bufferSize)
	err = pipe.Wait(n.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(runc)
	assert.Nil(t, err)

	// run while pausing
	runc = n.Run(bufferSize)
	pausec := n.Pause()
	// pause should cancel run channel
	err = pipe.Wait(runc)
	assert.Nil(t, err)
	// pausing
	err = pipe.Wait(n.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)
	_ = pipe.Wait(pausec)

	// run while paused
	err = pipe.Wait(n.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)

	// _ = pipe.Wait(n.Resume())
	_ = pipe.Wait(n.Close())
}

// Test Run method for all states.
func TestPause(t *testing.T) {
	n := newNet(t)
	// pause while ready
	errc := n.Pause()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	// pause while running
	_ = n.Run(bufferSize)
	errc = n.Pause()
	assert.NotNil(t, errc)
	err = pipe.Wait(errc)
	assert.Nil(t, err)

	// pause while pausing
	_ = n.Run(bufferSize)
	pausec := n.Pause()
	assert.NotNil(t, pausec)
	err = pipe.Wait(n.Pause())
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(pausec)

	// pause while paused
	err = pipe.Wait(n.Pause())
	assert.Equal(t, state.ErrInvalidState, err)
	_ = pipe.Wait(n.Resume())
	_ = pipe.Wait(n.Close())
}

// Test resume method for all states.
func TestResume(t *testing.T) {
	n := newNet(t)
	// resume while ready
	errc := n.Resume()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	// resume while running
	runc := n.Run(bufferSize)
	errc = n.Resume()
	err = pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(runc)

	// resume while paused
	_ = n.Run(bufferSize)
	pausec := n.Pause()
	_ = pipe.Wait(pausec)
	err = pipe.Wait(n.Resume())
	assert.Nil(t, err)
	_ = pipe.Wait(n.Close())
}

// To test leaks we need to call close method with all possible circumstances.
func TestLeaks(t *testing.T) {
	// close while ready
	n := newNet(t)
	err := pipe.Wait(n.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while running
	n = newNet(t)
	_ = n.Run(bufferSize)
	err = pipe.Wait(n.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while pausing
	n = newNet(t)
	_ = n.Run(bufferSize)
	_ = n.Pause()
	err = pipe.Wait(n.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while paused
	n = newNet(t)
	_ = n.Run(bufferSize)
	_ = pipe.Wait(n.Pause())
	err = pipe.Wait(n.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)
}

func TestClose(t *testing.T) {
	n := newNet(t)
	err := pipe.Wait(n.Close())
	assert.Nil(t, err)
}

// This is a constructor of test pipe
func newNet(t *testing.T) *pipe.Net {
	pump := &mock.Pump{
		// SampleRate:  44100,
		Limit:       5 * bufferSize,
		Interval:    10 * time.Microsecond,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}

	n := &pipe.Pipe{
		Pump:       pump,
		Processors: pipe.Processors(proc1, proc2),
		Sinks:      pipe.Sinks(sink1, sink2),
	}

	f, err := pipe.Network(
		n,
	)
	assert.Nil(t, err)
	return f
}

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
	p := newPipe(t)
	runc := p.Run(bufferSize)
	assert.NotNil(t, runc)
	err := pipe.Wait(runc)
	assert.Nil(t, err)

	pipe.Wait(p.Close())
}

// Test Run method for all states.
func TestRun(t *testing.T) {
	// run while ready
	p := newPipe(t)
	runc := p.Run(bufferSize)
	assert.NotNil(t, runc)
	err := pipe.Wait(runc)
	assert.Nil(t, err)

	// run while running
	runc = p.Run(bufferSize)
	err = pipe.Wait(p.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(runc)
	assert.Nil(t, err)

	// run while pausing
	runc = p.Run(bufferSize)
	pausec := p.Pause()
	// pause should cancel run channel
	err = pipe.Wait(runc)
	assert.Nil(t, err)
	// pausing
	err = pipe.Wait(p.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)
	_ = pipe.Wait(pausec)

	// run while paused
	err = pipe.Wait(p.Run(bufferSize))
	assert.Equal(t, state.ErrInvalidState, err)

	// _ = pipe.Wait(p.Resume())
	_ = pipe.Wait(p.Close())
}

// Test Run method for all states.
func TestPause(t *testing.T) {
	p := newPipe(t)
	// pause while ready
	errc := p.Pause()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	// pause while running
	_ = p.Run(bufferSize)
	errc = p.Pause()
	assert.NotNil(t, errc)
	err = pipe.Wait(errc)
	assert.Nil(t, err)

	// pause while pausing
	_ = p.Run(bufferSize)
	pausec := p.Pause()
	assert.NotNil(t, pausec)
	err = pipe.Wait(p.Pause())
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(pausec)

	// pause while paused
	err = pipe.Wait(p.Pause())
	assert.Equal(t, state.ErrInvalidState, err)
	_ = pipe.Wait(p.Resume())
	_ = pipe.Wait(p.Close())
}

// Test resume method for all states.
func TestResume(t *testing.T) {
	p := newPipe(t)
	// resume while ready
	errc := p.Resume()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)

	// resume while running
	runc := p.Run(bufferSize)
	errc = p.Resume()
	err = pipe.Wait(errc)
	assert.Equal(t, state.ErrInvalidState, err)
	err = pipe.Wait(runc)

	// resume while paused
	_ = p.Run(bufferSize)
	pausec := p.Pause()
	_ = pipe.Wait(pausec)
	err = pipe.Wait(p.Resume())
	assert.Nil(t, err)
	_ = pipe.Wait(p.Close())
}

// To test leaks we need to call close method with all possible circumstances.
func TestLeaks(t *testing.T) {
	// close while ready
	p := newPipe(t)
	err := pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while running
	p = newPipe(t)
	_ = p.Run(bufferSize)
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while pausing
	p = newPipe(t)
	_ = p.Run(bufferSize)
	_ = p.Pause()
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while paused
	p = newPipe(t)
	_ = p.Run(bufferSize)
	_ = pipe.Wait(p.Pause())
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)
}

// This is a constructor of test pipe
func newPipe(t *testing.T) *pipe.Net {
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

	p := pipe.Pipe{
		Pump:       pump,
		Processors: []pipe.Processor{proc1, proc2},
		Sinks:      []pipe.Sink{sink1, sink2},
	}

	f, err := pipe.Network(
		p,
	)
	assert.Nil(t, err)
	return f
}

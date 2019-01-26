package pipe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
	"go.uber.org/goleak"
)

const (
	bufferSize = 512
	sampleRate = 44100
)

var measureTests = struct {
	interval time.Duration
	mock.Limit
	BufferSize  int
	NumChannels int
}{
	interval:    10 * time.Millisecond,
	Limit:       10,
	BufferSize:  10,
	NumChannels: 1,
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// Test Run method for all states.
func TestRun(t *testing.T) {
	// run while ready
	p := newPipe(t)
	runc := p.Run()
	assert.NotNil(t, runc)
	err := pipe.Wait(runc)
	assert.Nil(t, err)

	// run while running
	runc = p.Run()
	err = pipe.Wait(p.Run())
	assert.Equal(t, pipe.ErrInvalidState, err)
	err = pipe.Wait(runc)
	assert.Nil(t, err)

	// run while pausing
	runc = p.Run()
	pausec := p.Pause()
	// pause should cancel run channel
	err = pipe.Wait(runc)
	assert.Nil(t, err)
	// pausing
	err = pipe.Wait(p.Run())
	assert.Equal(t, pipe.ErrInvalidState, err)
	_ = pipe.Wait(pausec)

	// run while paused
	err = pipe.Wait(p.Run())
	assert.Equal(t, pipe.ErrInvalidState, err)

	_ = pipe.Wait(p.Resume())
	_ = pipe.Wait(p.Close())
}

// Test Run method for all states.
func TestPause(t *testing.T) {
	p := newPipe(t)
	// pause while ready
	errc := p.Pause()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	assert.Equal(t, pipe.ErrInvalidState, err)

	// pause while running
	_ = p.Run()
	errc = p.Pause()
	assert.NotNil(t, errc)
	err = pipe.Wait(errc)
	assert.Nil(t, err)

	// pause while pausing
	_ = p.Run()
	pausec := p.Pause()
	assert.NotNil(t, pausec)
	err = pipe.Wait(p.Pause())
	assert.Equal(t, pipe.ErrInvalidState, err)
	err = pipe.Wait(pausec)

	// pause while paused
	err = pipe.Wait(p.Pause())
	assert.Equal(t, pipe.ErrInvalidState, err)
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
	assert.Equal(t, pipe.ErrInvalidState, err)

	// resume while running
	runc := p.Run()
	errc = p.Resume()
	err = pipe.Wait(errc)
	assert.Equal(t, pipe.ErrInvalidState, err)
	err = pipe.Wait(runc)

	// resume while paused
	_ = p.Run()
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
	_ = p.Run()
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while pausing
	p = newPipe(t)
	_ = p.Run()
	_ = p.Pause()
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)

	// close while paused
	p = newPipe(t)
	_ = p.Run()
	_ = pipe.Wait(p.Pause())
	err = pipe.Wait(p.Close())
	assert.Nil(t, err)
	goleak.VerifyNoLeaks(t)
}

// This is a constructor of test pipe
func newPipe(t *testing.T) *pipe.Pipe {
	pump := &mock.Pump{
		// UID:         phono.NewUID(),
		Limit:       5,
		Interval:    10 * time.Microsecond,
		BufferSize:  10,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}

	p, err := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	assert.Nil(t, err)
	return p
}

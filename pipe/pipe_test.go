package pipe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var (
	bufferSize = 512
)

func TestPipe(t *testing.T) {

	pump := &mock.Pump{
		Limit:    10,
		Interval: 0,
	}

	proc := &mock.Processor{}
	sink := &mock.Sink{}

	// new pipe
	p, err := pipe.New(
		pipe.WithPump(pump),
		pipe.WithProcessors(proc),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	// test wrong state for new pipe
	err = pipe.Wait(p.Pause())
	assert.NotNil(t, err)
	require.Equal(t, pipe.ErrInvalidState, err)

	// test pipe run
	err = pipe.Wait(p.Run())
	require.Nil(t, err)
	err = pipe.Wait(p.Signal.Running)
	require.Nil(t, err)

	// test push new opptions
	limit := mock.Limit(10)
	op := phono.NewParams().Add(pump.LimitParam(limit))
	p.Push(op)

	// test pipe pause
	err = pipe.Wait(p.Pause())
	require.Nil(t, err)
	err = pipe.Wait(p.Signal.Paused)
	require.Nil(t, err)

	// test pipe resume
	err = pipe.Wait(p.Resume())
	require.Nil(t, err)
	err = pipe.Wait(p.Signal.Running)
	err = pipe.Wait(p.Signal.Ready)

	// test rerun
	p.Push(op)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	require.Nil(t, err)
	err = pipe.Wait(p.Signal.Running)
	err = pipe.Wait(p.Signal.Ready)
	require.Nil(t, err)
}

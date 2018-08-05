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
		Limit:    1000,
		Interval: 0,
	}

	proc := &mock.Processor{}
	sink := &mock.Sink{}

	// new pipe
	p := pipe.New(
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc),
		pipe.WithSinks(sink),
	)

	// test wrong state for new pipe
	sig, err := p.Begin(pipe.Pause)
	assert.NotNil(t, err)
	require.Equal(t, pipe.ErrInvalidState, err)

	// test pipe run
	sig, err = p.Begin(pipe.Run)
	require.Nil(t, err)
	err = p.Wait(pipe.Running)
	require.Nil(t, err)

	// test push new opptions
	op := phono.NewParams(pump.LimitParam(100))
	p.Push(op)

	// time.Sleep(time.Millisecond * 10)
	// test pipe pause
	sig, err = p.Begin(pipe.Pause)
	require.Nil(t, err)
	err = p.Wait(sig)
	require.Nil(t, err)

	// test pipe resume
	sig, err = p.Begin(pipe.Resume)
	require.Nil(t, err)
	err = p.Wait(pipe.Running)
	err = p.Wait(pipe.Ready)

	// test rerun
	p.Push(op)
	assert.Nil(t, err)
	sig, err = p.Begin(pipe.Run)
	require.Nil(t, err)
	done := p.WaitAsync(sig)
	err = <-done
	require.Nil(t, err)
	p.Close()
}

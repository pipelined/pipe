package pipe_test

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"

// 	"github.com/dudk/phono"
// 	"github.com/dudk/phono/mock"
// 	"github.com/dudk/phono/pipe"
// )

// var (
// 	bufferSize = 512
// )

// func TestPipe(t *testing.T) {

// 	pump := &mock.Pump{
// 		Limit:    10,
// 		Interval: 0,
// 	}

// 	proc := &mock.Processor{}
// 	sink := &mock.Sink{}

// 	// new pipe
// 	p := pipe.New(
// 		pipe.WithPump(pump),
// 		pipe.WithProcessors(proc),
// 		pipe.WithSinks(sink),
// 	)

// 	// test wrong state for new pipe
// 	err := pipe.Do(p.Pause)
// 	assert.NotNil(t, err)
// 	require.Equal(t, pipe.ErrInvalidState, err)
// 	fmt.Println("1")

// 	// test pipe run
// 	err = pipe.Do(p.Run)
// 	require.Nil(t, err)
// 	err = p.Wait(pipe.Running)
// 	require.Nil(t, err)

// 	// test push new opptions
// 	limit := mock.Limit(10)
// 	op := phono.NewParams().Add(pump.LimitParam(limit))
// 	p.Push(op)

// 	// time.Sleep(time.Millisecond * 10)
// 	// test pipe pause
// 	err = pipe.Do(p.Pause)
// 	require.Nil(t, err)
// 	err = p.Wait(pipe.Paused)
// 	require.Nil(t, err)

// 	// test pipe resume
// 	err = pipe.Do(p.Resume)
// 	require.Nil(t, err)
// 	err = p.Wait(pipe.Running)
// 	err = p.Wait(pipe.Ready)

// 	// test rerun
// 	p.Push(op)
// 	assert.Nil(t, err)
// 	err = pipe.Do(p.Run)
// 	require.Nil(t, err)
// 	// err = p.Wait(pipe.Running)
// 	err = p.Wait(pipe.Ready)
// 	require.Nil(t, err)
// 	p.Close()
// }

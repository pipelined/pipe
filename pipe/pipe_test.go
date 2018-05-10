package pipe_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var (
	bufferSize = 512
)

func TestPipe(t *testing.T) {

	pump := &mock.Pump{
		OptionUser: &mock.OptionUser{},
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
	err = p.Validate()
	assert.NotNil(t, err)

	proc.Simple = 100
	err = p.Validate()
	assert.Nil(t, err)

	// test wrong state pipe
	done, err := p.Pause()
	assert.NotNil(t, err)
	assert.Equal(t, pipe.ErrInvalidState, err)

	done, err = p.Run()
	assert.Nil(t, err)
	// err = pipe.Wait(done)
	// assert.Nil(t, err)

	// time.Sleep(time.Second * 1)

	s := pump.WithSimple(mock.SimpleOption(10))
	fmt.Println("sending options")
	op := phono.NewOptions().AddOptionsFor(pump, s)
	p.Push(op)
	fmt.Println("options are sent")

	done, err = p.Pause()
	assert.Nil(t, err)
	fmt.Println("waiting paused")
	err = pipe.Wait(done)
	assert.Nil(t, err)
	fmt.Println("done paused")

	done, err = p.Resume()
	assert.Nil(t, err)
	fmt.Println("waiting resume")
	err = pipe.Wait(done)
	assert.Nil(t, err)

	// assert.Nil(t, err)
	// fmt.Println("waiting pipe")
	// err = pipe.Wait(done)
	// fmt.Println("pipe is done")
	// assert.Nil(t, err)

	err = p.Push(op)
	assert.Nil(t, err)
	done, err = p.Run()
	assert.Nil(t, err)
	pipe.Wait(done)
	// err = pipe.Wait(p.Signal.Interrupted)
}

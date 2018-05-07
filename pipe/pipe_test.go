package pipe_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var (
	bufferSize = 512
)

// TODO: build tests with mock package

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
	err = pipe.Wait(done)
	assert.Nil(t, err)

	time.Sleep(time.Second * 1)

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

	// time.Sleep(time.Second * 1)

	done, err = p.Resume()
	assert.Nil(t, err)
	fmt.Println("waiting resume")
	err = pipe.Wait(done)
	assert.Nil(t, err)

	assert.Nil(t, err)
	fmt.Println("waiting pipe")
	err = pipe.Wait(p.Interrupt)
	fmt.Println("pipe is done")
	assert.Nil(t, err)
}

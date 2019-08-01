package pipe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/mock"
	"github.com/pipelined/pipe"
	"go.uber.org/goleak"
)

const (
	bufferSize = 512
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNetwork(t *testing.T) {
	pump := &mock.Pump{
		Limit:       100 * bufferSize,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}

	n, err := pipe.Network(
		&pipe.Pipe{
			Pump:       pump,
			Processors: pipe.Processors(proc1, proc2),
			Sinks:      pipe.Sinks(sink1, sink2),
		},
	)
	assert.Nil(t, err)

	// start the net
	runc := n.Run(bufferSize)
	assert.NotNil(t, runc)
	assert.Nil(t, err)

	// test params push
	pumpID, ok := n.ComponentID(pump)
	assert.True(t, ok)
	assert.NotEmpty(t, pumpID)
	//TODO: ADD PUSH
	// push new limit for pump
	newLimit := 200
	paramFn := pump.LimitParam(newLimit)
	n.Push(pumpID, paramFn)

	// pause the net
	err = pipe.Wait(n.Pause())
	assert.Nil(t, err)
	// runc must be cancelled by now
	err = pipe.Wait(runc)
	assert.Nil(t, err)

	// resume the net
	err = pipe.Wait(n.Resume())
	assert.Nil(t, err)

	pipe.Wait(n.Close())
}

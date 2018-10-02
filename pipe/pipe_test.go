package pipe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

const (
	bufferSize = 512
	sampleRate = 44100
)

func TestPipeActions(t *testing.T) {

	pump := &mock.Pump{
		Limit:       1000,
		Interval:    0,
		NumChannels: 1,
	}

	proc := &mock.Processor{}
	sink := &mock.Sink{}

	// new pipe
	p := pipe.New(
		sampleRate,
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

func TestPipe(t *testing.T) {
	messages := int64(1000)
	samples := int64(10)
	pump := &mock.Pump{
		Limit:       mock.Limit(messages),
		Interval:    0,
		BufferSize:  phono.BufferSize(samples),
		NumChannels: 1,
	}

	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}
	// new pipe
	p := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	err := p.Do(pipe.Run)
	assert.Nil(t, err)

	messageCount, samplesCount := pump.Count()
	assert.Equal(t, messages, messageCount)
	assert.Equal(t, samples*messages, samplesCount)

	messageCount, samplesCount = proc1.Count()
	assert.Equal(t, messages, messageCount)
	assert.Equal(t, samples*messages, samplesCount)

	messageCount, samplesCount = sink1.Count()
	assert.Equal(t, messages, messageCount)
	assert.Equal(t, samples*messages, samplesCount)
	p.Close()
	p.Close()
}

package pipe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

const (
	bufferSize                  = 512
	sampleRate phono.SampleRate = 44100
)

var measureTests = struct {
	interval time.Duration
	mock.Limit
	phono.BufferSize
	phono.NumChannels
}{
	interval:    10 * time.Millisecond,
	Limit:       10,
	BufferSize:  10,
	NumChannels: 1,
}

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
	p.Push(pump.LimitParam(200))

	// time.Sleep(time.Millisecond * 10)
	// test pipe pause
	sig, err = p.Begin(pipe.Pause)
	require.Nil(t, err)
	err = p.Wait(sig)
	require.Nil(t, err)

	mc := p.Measure(pump.ID())
	measure := <-mc
	assert.NotNil(t, measure)

	// test pipe resume
	sig, err = p.Begin(pipe.Resume)
	require.Nil(t, err)
	err = p.Wait(pipe.Running)
	err = p.Wait(pipe.Ready)

	// test rerun
	p.Push(pump.LimitParam(200))
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
	mc := p.Measure(pump.ID())
	measure := <-mc
	assert.NotNil(t, measure)
	p.Close()
	p.Close()
}

func TestMetricsEmpty(t *testing.T) {
	p := pipe.New(sampleRate)
	mc := p.Measure()
	assert.Nil(t, mc)
}

func TestMetricsBadID(t *testing.T) {
	proc := &mock.Processor{}
	p := pipe.New(sampleRate, pipe.WithProcessors(proc))
	mc := p.Measure(proc.ID() + "bad")
	assert.Nil(t, mc)
}

func TestMetrics(t *testing.T) {
	pump := &mock.Pump{
		Limit:       measureTests.Limit,
		Interval:    measureTests.interval,
		BufferSize:  measureTests.BufferSize,
		NumChannels: measureTests.NumChannels,
	}
	proc := &mock.Processor{}
	sink := &mock.Sink{}
	p := pipe.New(
		sampleRate,
		pipe.WithName("Test Metrics"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc),
		pipe.WithSinks(sink),
	)

	var mc <-chan pipe.Measure

	// zero measures
	mc = p.Measure()
	assert.NotNil(t, mc)
	for m := range mc {
		assert.NotNil(t, m)
		assert.Equal(t, time.Time{}, m.Start)
		assert.Equal(t, time.Duration(0), m.Elapsed)
		for _, c := range m.Counters {
			assert.Equal(t, int64(0), c.Messages())
			assert.Equal(t, int64(0), c.Samples())
			assert.Equal(t, time.Duration(0), c.Duration())
		}
		switch m.ID {
		case pump.ID():
		case proc.ID():
		case sink.ID():
		default:
			t.Errorf("Measure with empty id")
		}
	}

	start := time.Now()
	p.Begin(pipe.Run)
	time.Sleep(measureTests.interval / 2)
	p.Begin(pipe.Pause)
	mc = p.Measure(pump.ID(), proc.ID(), sink.ID())
	// measure diring pausing
	for m := range mc {
		assert.NotNil(t, m)
		assert.True(t, start.Before(m.Start))
		assert.True(t, measureTests.interval < m.Elapsed)
		for _, c := range m.Counters {
			assert.Equal(t, int64(2), c.Messages())
			assert.Equal(t, int64(measureTests.BufferSize)*2, c.Samples())
			assert.Equal(t, sampleRate.DurationOf(int64(measureTests.BufferSize))*2, c.Duration())
		}
		switch m.ID {
		case pump.ID():
		case proc.ID():
		case sink.ID():
		default:
			t.Errorf("Measure with empty id")
		}
	}
}

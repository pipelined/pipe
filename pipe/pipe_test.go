package pipe_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
	"go.uber.org/goleak"
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

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPipeActions(t *testing.T) {
	pump := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       1000,
		Interval:    0,
		NumChannels: 1,
	}

	proc := &mock.Processor{UID: phono.NewUID()}
	sink := &mock.Sink{UID: phono.NewUID()}

	// new pipe
	p := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc),
		pipe.WithSinks(sink),
	)

	// test wrong state for new pipe
	errc := p.Pause()
	assert.NotNil(t, errc)
	err := pipe.Wait(errc)
	require.Equal(t, pipe.ErrInvalidState, err)

	// test pipe run
	errc = p.Run(context.Background())
	require.NotNil(t, errc)

	// test push new opptions
	p.Push(pump.LimitParam(200))

	// time.Sleep(time.Millisecond * 10)
	// test pipe pause
	errc = p.Pause()
	require.NotNil(t, err)
	err = pipe.Wait(errc)
	require.Nil(t, err)

	mc := p.Measure(pump.ID())
	measure := <-mc
	assert.NotNil(t, measure)

	// test pipe resume
	errc = p.Resume()
	require.NotNil(t, errc)
	err = pipe.Wait(errc)
	require.Nil(t, err)

	// test rerun
	p.Push(pump.LimitParam(200))
	assert.Nil(t, err)
	errc = p.Run(context.Background())
	require.NotNil(t, errc)
	err = pipe.Wait(errc)
	require.Nil(t, err)
	p.Close()
}

func TestPipe(t *testing.T) {
	messages := int64(1000)
	samples := int64(10)
	pump := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       mock.Limit(messages),
		Interval:    0,
		BufferSize:  phono.BufferSize(samples),
		NumChannels: 1,
	}

	proc1 := &mock.Processor{UID: phono.NewUID()}
	proc2 := &mock.Processor{UID: phono.NewUID()}
	sink1 := &mock.Sink{UID: phono.NewUID()}
	sink2 := &mock.Sink{UID: phono.NewUID()}
	// new pipe
	p := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	err := pipe.Wait(p.Run(context.Background()))
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
	p.Close()
}

func TestMetricsBadID(t *testing.T) {
	proc := &mock.Processor{UID: phono.NewUID()}
	p := pipe.New(sampleRate, pipe.WithProcessors(proc))
	mc := p.Measure(proc.ID() + "bad")
	assert.Nil(t, mc)
	p.Close()
}

func TestMetrics(t *testing.T) {
	pump := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       measureTests.Limit,
		Interval:    measureTests.interval,
		BufferSize:  measureTests.BufferSize,
		NumChannels: measureTests.NumChannels,
	}
	proc := &mock.Processor{UID: phono.NewUID()}
	sink := &mock.Sink{UID: phono.NewUID()}
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
	p.Run(context.Background())
	time.Sleep(measureTests.interval / 2)
	err := pipe.Wait(p.Pause())
	assert.Nil(t, err)
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
	err = pipe.Wait(p.Resume())
	assert.Nil(t, err)
	p.Close()
}

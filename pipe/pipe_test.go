package pipe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

func TestPipe(t *testing.T) {
	pump := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       5,
		Interval:    10 * time.Microsecond,
		BufferSize:  10,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{UID: phono.NewUID()}
	proc2 := &mock.Processor{UID: phono.NewUID()}
	sink1 := &mock.Sink{UID: phono.NewUID()}
	sink2 := &mock.Sink{UID: phono.NewUID()}
	p := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)

	testRun(t, p)
	testPause(t, p)
	testResume(t, p)
	p.Close()
}

// Test Run method for all states.
func testRun(t *testing.T, p *pipe.Pipe) {
	// run while ready
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
}

// Test Run method for all states.
func testPause(t *testing.T, p *pipe.Pipe) {
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
}

// Test resume method for all states.
func testResume(t *testing.T, p *pipe.Pipe) {
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
}

// To test leaks we need to call close method with all possible circumstances.
func TestLeaks(t *testing.T) {
	pump := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       5,
		Interval:    10 * time.Microsecond,
		BufferSize:  10,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{UID: phono.NewUID()}
	proc2 := &mock.Processor{UID: phono.NewUID()}
	sink1 := &mock.Sink{UID: phono.NewUID()}
	sink2 := &mock.Sink{UID: phono.NewUID()}
	p := pipe.New(
		sampleRate,
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)

	// start the test
	_ = p.Run()
	p.Close()
	goleak.VerifyNoLeaks(t)
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
	p.Run()
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

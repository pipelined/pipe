package runner_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe"
	"github.com/pipelined/signal"

	"github.com/pipelined/pipe/internal/mock"
	"github.com/pipelined/pipe/internal/runner"
	"github.com/pipelined/pipe/metric"
)

const (
	pipeID      = "testPipeID"
	componentID = "testComponentID"
)

type noOpPool struct {
	numChannels int
	bufferSize  int
}

func (p noOpPool) Alloc() signal.Float64 {
	return signal.Float64Buffer(p.numChannels, p.bufferSize)
}

func (p noOpPool) Free(signal.Float64) {}

var testError = errors.New("Test runner error")

func TestPumpRunner(t *testing.T) {
	bufferSize := 1024
	tests := []struct {
		cancelOnGive bool
		cancelOnTake bool
		cancelOnSend bool
		pump         *mock.Pump
	}{
		{
			pump: &mock.Pump{
				NumChannels: 1,
				Limit:       10 * bufferSize,
			},
		},
		{
			cancelOnGive: true,
			pump: &mock.Pump{
				NumChannels: 1,
			},
		},
		{
			cancelOnTake: true,
			pump: &mock.Pump{
				NumChannels: 1,
			},
		},
		// This test case cannot guarantee coverage because buffered out channel is used.
		// {
		// 	cancelOnSend: true,
		// 	pump: &mock.Pump{
		// 		NumChannels: 1,
		// 		Limit:       bufferSize,
		// 	},
		// },
		{
			pump: &mock.Pump{
				ErrorOnCall: testError,
				NumChannels: 1,
				Limit:       bufferSize,
			},
		},
		{
			pump: &mock.Pump{
				Hooks: mock.Hooks{
					ErrorOnReset: testError,
				},
				NumChannels: 1,
				Limit:       bufferSize,
			},
		},
	}

	var ok bool

	for _, c := range tests {
		fn, sampleRate, _, _ := c.pump.Pump(pipeID)
		r := runner.Pump{
			Fn:    fn,
			Meter: metric.Meter(c.pump, signal.SampleRate(sampleRate)),
			Hooks: pipe.BindHooks(c.pump),
		}
		cancelc := make(chan struct{})
		givec := make(chan string)
		takec := make(chan runner.Message)
		out, errc := r.Run(
			noOpPool{
				numChannels: c.pump.NumChannels,
				bufferSize:  bufferSize,
			},
			pipeID,
			componentID,
			cancelc,
			givec,
			takec,
		)
		assert.NotNil(t, out)
		assert.NotNil(t, errc)

		// test cancellation
		switch {
		case c.cancelOnGive:
			close(cancelc)
		case c.cancelOnTake:
			<-givec
			close(cancelc)
		case c.cancelOnSend:
			<-givec
			takec <- runner.Message{
				PipeID: pipeID,
			}
			close(cancelc)
		case c.pump.ErrorOnCall != nil:
			<-givec
			takec <- runner.Message{
				PipeID: pipeID,
			}
			<-out
			err := <-errc
			assert.Equal(t, c.pump.ErrorOnCall, err)
		case c.pump.ErrorOnReset != nil:
			err := <-errc
			assert.Equal(t, c.pump.ErrorOnReset, err)
		default:
			// test message exchange
			for i := 0; i <= c.pump.Limit/bufferSize; i++ {
				<-givec
				takec <- runner.Message{
					PipeID: pipeID,
				}
				<-out
			}
		}

		pipe.Wait(errc)

		// test channels closed
		_, ok = <-out
		assert.False(t, ok)
		_, ok = <-errc
		assert.False(t, ok)

		assert.True(t, c.pump.Resetted)

		if c.pump.ErrorOnReset != nil {
			assert.False(t, c.pump.Flushed)
		} else {
			assert.True(t, c.pump.Flushed)
		}

		if c.cancelOnGive || c.cancelOnTake || c.cancelOnSend {
			assert.True(t, c.pump.Interrupted)
		} else {
			assert.False(t, c.pump.Interrupted)
		}
	}
}

func TestProcessorRunner(t *testing.T) {
	tests := []struct {
		messages        int
		cancelOnReceive bool
		cancelOnSend    bool
		processor       *mock.Processor
	}{
		{
			messages:  10,
			processor: &mock.Processor{},
		},
		{
			processor: &mock.Processor{
				ErrorOnCall: testError,
			},
		},
		{
			cancelOnReceive: true,
			processor:       &mock.Processor{},
		},
		// This test case cannot guarantee coverage because buffered out channel is used.
		// {
		// 	cancelOnSend: true,
		// 	processor:    &mock.Processor{},
		// },
		{
			processor: &mock.Processor{
				Hooks: mock.Hooks{
					ErrorOnReset: testError,
				},
			},
		},
	}
	sampleRate := signal.SampleRate(44100)
	numChannels := 1
	for _, c := range tests {
		fn, _ := c.processor.Process(pipeID, sampleRate, numChannels)
		r := runner.Processor{
			Fn:    fn,
			Meter: metric.Meter(c.processor, signal.SampleRate(sampleRate)),
			Hooks: pipe.BindHooks(c.processor),
		}

		cancelc := make(chan struct{})
		in := make(chan runner.Message)
		out, errc := r.Run(pipeID, componentID, cancelc, in)
		assert.NotNil(t, out)
		assert.NotNil(t, errc)

		switch {
		case c.cancelOnReceive:
			close(cancelc)
		case c.cancelOnSend:
			in <- runner.Message{
				PipeID: pipeID,
			}
			close(cancelc)
		case c.processor.ErrorOnCall != nil:
			in <- runner.Message{
				PipeID: pipeID,
			}
			err := <-errc
			assert.Equal(t, c.processor.ErrorOnCall, err)
		case c.processor.ErrorOnReset != nil:
			err := <-errc
			assert.Equal(t, c.processor.ErrorOnReset, err)
		default:
			for i := 0; i <= c.messages; i++ {
				in <- runner.Message{
					PipeID: pipeID,
				}
				<-out
			}
			close(in)
		}

		pipe.Wait(errc)

		assert.True(t, c.processor.Resetted)
		if c.processor.ErrorOnReset != nil {
			assert.False(t, c.processor.Flushed)
		} else {
			assert.True(t, c.processor.Flushed)
		}

		switch {
		case c.cancelOnReceive || c.cancelOnSend:
			assert.True(t, c.processor.Interrupted)
		default:
			assert.False(t, c.processor.Interrupted)
		}
	}
}

func TestSinkRunner(t *testing.T) {
	tests := []struct {
		messages        int
		cancelOnReceive bool
		sink            *mock.Sink
	}{
		{
			messages: 10,
			sink:     &mock.Sink{},
		},
		{
			sink: &mock.Sink{
				ErrorOnCall: testError,
			},
		},
		{
			cancelOnReceive: true,
			sink:            &mock.Sink{},
		},
		{
			sink: &mock.Sink{
				Hooks: mock.Hooks{
					ErrorOnReset: testError,
				},
			},
		},
	}

	sampleRate := signal.SampleRate(44100)
	numChannels := 1
	for _, c := range tests {
		fn, _ := c.sink.Sink(pipeID, sampleRate, numChannels)

		r := runner.Sink{
			Fn:    fn,
			Meter: metric.Meter(c.sink, signal.SampleRate(sampleRate)),
			Hooks: pipe.BindHooks(c.sink),
		}

		cancelc := make(chan struct{})
		in := make(chan runner.Message)
		errc := r.Run(noOpPool{}, pipeID, componentID, cancelc, in)
		assert.NotNil(t, errc)

		switch {
		case c.cancelOnReceive:
			close(cancelc)
		case c.sink.ErrorOnCall != nil:
			in <- runner.Message{
				PipeID: pipeID,
			}
			err := <-errc
			assert.Equal(t, c.sink.ErrorOnCall, err)
		case c.sink.ErrorOnReset != nil:
			err := <-errc
			assert.Equal(t, c.sink.ErrorOnReset, err)
		default:
			for i := 0; i <= c.messages; i++ {
				in <- runner.Message{
					SinkRefs: 1,
					PipeID:   pipeID,
				}
			}
			close(in)
		}

		pipe.Wait(errc)

		assert.True(t, c.sink.Resetted)
		if c.sink.ErrorOnReset != nil {
			assert.False(t, c.sink.Flushed)
		} else {
			assert.True(t, c.sink.Flushed)
		}

		switch {
		case c.cancelOnReceive:
			assert.True(t, c.sink.Interrupted)
		default:
			assert.False(t, c.sink.Interrupted)
		}
	}
}

func TestBroadcast(t *testing.T) {
	tests := []struct {
		sinks    []pipe.Sink
		messages int
	}{
		{
			sinks: []pipe.Sink{
				&mock.Sink{},
				&mock.Sink{},
			},
			messages: 10,
		},
	}
	sampleRate := signal.SampleRate(44100)
	numChannels := 1
	for _, test := range tests {

		// create runners
		runners := make([]runner.Sink, len(test.sinks))
		for i, sink := range test.sinks {
			fn, _ := sink.Sink(pipeID, sampleRate, numChannels)
			r := runner.Sink{
				Fn:    fn,
				Meter: metric.Meter(sink, sampleRate),
				Hooks: pipe.BindHooks(sink),
			}
			runners[i] = r
		}

		cancelChan := make(chan struct{})
		inChan := make(chan runner.Message)
		errChanList := runner.Broadcast(
			noOpPool{},
			pipeID,
			runners,
			cancelChan,
			inChan,
		)
		assert.Equal(t, len(runners), len(errChanList))
		for i := 0; i < test.messages; i++ {
			inChan <- runner.Message{
				PipeID: pipeID,
			}
		}
		close(inChan)
		// _, ok := <-cancelChan
		// assert.False(t, ok)
		for _, errChan := range errChanList {
			_, ok := <-errChan
			assert.False(t, ok)
		}
	}
}

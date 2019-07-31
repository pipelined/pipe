package runner_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/mock"
	"github.com/pipelined/pipe"
	"github.com/pipelined/pipe/metric"

	"github.com/pipelined/pipe/internal/runner"
)

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
		{
			cancelOnSend: true,
			pump: &mock.Pump{
				NumChannels: 1,
				Limit:       bufferSize,
			},
		},
		{
			pump: &mock.Pump{
				ErrorOnSend: testError,
				NumChannels: 1,
				Limit:       bufferSize,
			},
		},
	}
	pipeID := "testPipeID"
	componentID := "testComponentID"

	var ok bool

	for _, c := range tests {
		fn, sampleRate, _, _ := c.pump.Pump(pipeID)
		r := &runner.Pump{
			Fn:    fn,
			Meter: metric.Meter(c.pump, sampleRate),
			Hooks: runner.BindHooks(c.pump),
		}
		cancelc := make(chan struct{})
		givec := make(chan string)
		takec := make(chan runner.Message)
		out, errc := r.Run(bufferSize, pipeID, componentID, cancelc, givec, takec)
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
				SourceID: pipeID,
			}
			close(cancelc)
		case c.pump.ErrorOnSend != nil:
			<-givec
			takec <- runner.Message{
				SourceID: pipeID,
			}
			<-out
			err := <-errc
			assert.Equal(t, c.pump.ErrorOnSend, err)
		default:
			// test message exchange
			for i := 0; i <= c.pump.Limit/bufferSize; i++ {
				<-givec
				takec <- runner.Message{
					SourceID: pipeID,
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

		switch {
		case c.pump.ErrorOnSend != nil:
			assert.False(t, c.pump.Interrupted)
			assert.False(t, c.pump.Flushed)
		case c.cancelOnGive || c.cancelOnTake || c.cancelOnSend:
			assert.True(t, c.pump.Interrupted)
			assert.False(t, c.pump.Flushed)
		default:
			assert.False(t, c.pump.Interrupted)
			assert.True(t, c.pump.Flushed)
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
				ErrorOnSend: testError,
			},
		},
		{
			cancelOnReceive: true,
			processor:       &mock.Processor{},
		},
		{
			cancelOnSend: true,
			processor:    &mock.Processor{},
		},
	}
	pipeID := "testPipeID"
	componentID := "testComponentID"
	sampleRate := 44100
	numChannels := 1
	for _, c := range tests {
		fn, _ := c.processor.Process(pipeID, sampleRate, numChannels)
		r := &runner.Processor{
			Fn:    fn,
			Meter: metric.Meter(c.processor, sampleRate),
			Hooks: runner.BindHooks(c.processor),
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
				SourceID: pipeID,
			}
			close(cancelc)
		case c.processor.ErrorOnSend != nil:
			in <- runner.Message{
				SourceID: pipeID,
			}
			err := <-errc
			assert.Equal(t, c.processor.ErrorOnSend, err)
		default:
			for i := 0; i <= c.messages; i++ {
				in <- runner.Message{
					SourceID: pipeID,
				}
				<-out
			}
			close(in)
		}

		pipe.Wait(errc)

		assert.True(t, c.processor.Resetted)
		switch {
		case c.processor.ErrorOnSend != nil:
			assert.False(t, c.processor.Flushed)
			assert.False(t, c.processor.Interrupted)
		case c.cancelOnReceive || c.cancelOnSend:
			assert.False(t, c.processor.Flushed)
			assert.True(t, c.processor.Interrupted)
		default:
			assert.True(t, c.processor.Flushed)
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
				ErrorOnSend: testError,
			},
		},
		{
			cancelOnReceive: true,
			sink:            &mock.Sink{},
		},
	}
	pipeID := "testPipeID"
	componentID := "testComponentID"
	sampleRate := 44100
	numChannels := 1
	for _, c := range tests {
		fn, _ := c.sink.Sink(pipeID, sampleRate, numChannels)

		r := &runner.Sink{
			Fn:    fn,
			Meter: metric.Meter(c.sink, sampleRate),
			Hooks: runner.BindHooks(c.sink),
		}

		cancelc := make(chan struct{})
		in := make(chan runner.Message)
		errc := r.Run(pipeID, componentID, cancelc, in)
		assert.NotNil(t, errc)

		switch {
		case c.cancelOnReceive:
			close(cancelc)
		// case c.cancelOnSend:
		// 	in <- runner.Message{
		// 		SourceID: pipeID,
		// 	}
		// 	close(cancelc)
		case c.sink.ErrorOnSend != nil:
			in <- runner.Message{
				SourceID: pipeID,
			}
			err := <-errc
			assert.Equal(t, c.sink.ErrorOnSend, err)
		default:
			for i := 0; i <= c.messages; i++ {
				in <- runner.Message{
					SourceID: pipeID,
				}
			}
			close(in)
		}

		pipe.Wait(errc)

		assert.True(t, c.sink.Resetted)
		switch {
		case c.sink.ErrorOnSend != nil:
			assert.False(t, c.sink.Flushed)
			assert.False(t, c.sink.Interrupted)
		case c.cancelOnReceive:
			assert.False(t, c.sink.Flushed)
			assert.True(t, c.sink.Interrupted)
		default:
			assert.True(t, c.sink.Flushed)
			assert.False(t, c.sink.Interrupted)
		}
	}
}

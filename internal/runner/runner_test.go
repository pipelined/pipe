package runner_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/mock"
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

		// test channels closed
		_, ok = <-out
		assert.False(t, ok)
		_, ok = <-errc
		assert.False(t, ok)

		assert.True(t, c.pump.Resetted)

		if c.pump.ErrorOnSend != nil {
			assert.False(t, c.pump.Interrupted)
			assert.False(t, c.pump.Flushed)
		} else if c.cancelOnGive || c.cancelOnTake || c.cancelOnSend {
			assert.True(t, c.pump.Interrupted)
			assert.False(t, c.pump.Flushed)
		} else {
			assert.False(t, c.pump.Interrupted)
			assert.True(t, c.pump.Flushed)
		}
	}
}

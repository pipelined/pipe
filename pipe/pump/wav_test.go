package pump

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWavPump(t *testing.T) {
	reader, err := NewWav("../../_testdata/test.wav", 512)
	assert.Nil(t, err)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, errorc, err := reader.Pump(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 44100, reader.SampleRate)
	assert.Equal(t, 16, reader.BitDepth)
	assert.Equal(t, 2, reader.NumChannels)
	samplesRead, bufCount := 0, 0
	for out != nil {
		select {
		case buf, ok := <-out:
			if !ok {
				out = nil
			} else {
				samplesRead = samplesRead + buf.Size()
				bufCount++
			}
		case err = <-errorc:
			assert.Nil(t, err)
		}

	}
	assert.Equal(t, 646, bufCount)
	assert.Equal(t, 330534, samplesRead)
}

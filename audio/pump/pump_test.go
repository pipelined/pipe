package pump

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWavPump(t *testing.T) {
	reader := Wav{
		Path:       "../../_testdata/test.wav",
		BufferSize: 512,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, errorc, err := reader.Pump(ctx)
	assert.Nil(t, err)
	var samplesRead, bufCount int
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
			if err != nil {
				fmt.Printf("Error recieved: %v\n", err)
			}
		}

	}

	fmt.Printf("Buffers read: %d \nSamples read: %d\n", bufCount, samplesRead)
}

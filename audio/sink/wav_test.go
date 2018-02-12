package sink

import (
	"context"
	"fmt"
	"testing"

	"github.com/dudk/phono/audio/pump"
	"github.com/stretchr/testify/assert"
)

func TestWavSink(t *testing.T) {
	pump := pump.Wav{
		Path:       "../../_testdata/test.wav",
		BufferSize: 512,
	}
	sink := Wav{
		Path:       "../../_testdata/out.wav",
		BufferSize: 512,
		BitDepth:   16,
		SampleRate: 44100,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, _, err := pump.Pump(ctx)
	assert.Nil(t, err)

	errorc, err := sink.Sink(ctx, out)
	assert.Nil(t, err)
	for err = range errorc {
		fmt.Printf("Error waiting for sink: %v", err)
	}
	assert.Nil(t, err)
}

func TestWavSink2(t *testing.T) {
	pump := pump.Wav{
		Path:       "../../_testdata/test.wav",
		BufferSize: 512,
	}
	sink := Wav{
		Path:       "../../_testdata/out2.wav",
		BufferSize: 512,
		BitDepth:   16,
		SampleRate: 44100,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	out, _, err := pump.PumpNew(ctx)
	assert.Nil(t, err)

	errorc, err := sink.Sink(ctx, out)
	assert.Nil(t, err)
	for err = range errorc {
		fmt.Printf("Error waiting for sink: %v", err)
	}
	assert.Nil(t, err)
}

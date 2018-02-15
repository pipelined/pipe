package pipe

import (
	"context"
	"testing"

	"github.com/dudk/phono/pipe/pump"
	"github.com/dudk/phono/pipe/sink"
	"github.com/stretchr/testify/assert"
)

var (
	inFile     = "../_testdata/test.wav"
	outFile    = "../_testdata/out.wav"
	outFile2   = "../_testdata/out2.wav"
	bufferSize = 512
)

func TestPipe(t *testing.T) {
	wavPump, err := pump.NewWav(inFile, bufferSize)
	assert.Nil(t, err)
	wavSink := sink.NewWav(
		outFile,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	wavSink1 := sink.NewWav(
		outFile2,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	pipe := New(
		Pump(*wavPump),
		Sinks(*wavSink, *wavSink1),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err = pipe.Run(ctx)
	assert.Nil(t, err)
}

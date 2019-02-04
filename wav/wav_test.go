package wav_test

import (
	"math"
	"testing"

	"github.com/pipelined/phono"

	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/signal"
	"github.com/pipelined/phono/test"

	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/wav"
	"github.com/stretchr/testify/assert"
)

const (
	bufferSize = 512
)

func TestWavPipe(t *testing.T) {
	tests := []struct {
		inFile   string
		outFile  string
		messages int
		samples  int
	}{
		{
			inFile:   test.Data.Wav1,
			outFile:  test.Out.Wav1,
			messages: int(math.Ceil(float64(test.Data.Wav1Samples) / float64(bufferSize))),
			samples:  test.Data.Wav1Samples,
		},
		{
			inFile:   test.Out.Wav1,
			outFile:  test.Out.Wav2,
			messages: int(math.Ceil(float64(test.Data.Wav1Samples) / float64(bufferSize))),
			samples:  test.Data.Wav1Samples,
		},
	}

	for _, test := range tests {
		pump := wav.NewPump(test.inFile)
		sink, err := wav.NewSink(test.outFile, signal.BitDepth16)
		assert.Nil(t, err)

		processor := &mock.Processor{}
		p, err := pipe.New(
			bufferSize,
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		assert.Nil(t, err)
		err = pipe.Wait(p.Run())
		assert.Nil(t, err)
		messageCount, sampleCount := processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, sampleCount)

		err = pipe.Wait(p.Run())
		assert.Equal(t, phono.ErrSingleUseReused, err)
	}
}

func TestWavPumpErrors(t *testing.T) {
	tests := []struct {
		path string
	}{
		{
			path: "non-existing file",
		},
		{
			path: test.Data.Mp3,
		},
		{
			path: test.Data.Wav8Bit,
		},
	}

	for _, test := range tests {
		pump := wav.NewPump(test.path)
		_, _, _, err := pump.Pump("", 0)
		assert.NotNil(t, err)
	}
}

func TestWavSinkErrors(t *testing.T) {
	// test unsupported bit depth
	_, err := wav.NewSink("test", signal.BitDepth8)
	assert.NotNil(t, err)

	// test empty file name
	sink, err := wav.NewSink("", signal.BitDepth16)
	assert.Nil(t, err)
	_, err = sink.Sink("test", 0, 0, 0)
	assert.NotNil(t, err)
}

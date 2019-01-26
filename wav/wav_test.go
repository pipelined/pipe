package wav_test

import (
	"math"
	"testing"

	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/test"

	"github.com/pipelined/phono"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/wav"
	"github.com/stretchr/testify/assert"
)

const (
	bufferSize = 512
)

var tests = []struct {
	BufferSize int
	inFile     string
	outFile    string
	messages   int
	samples    int
}{
	{
		BufferSize: bufferSize,
		inFile:     test.Data.Wav1,
		outFile:    test.Out.Wav1,
		messages:   int(math.Ceil(float64(test.Data.Wav1Samples) / float64(bufferSize))),
		samples:    test.Data.Wav1Samples,
	},
	{
		BufferSize: bufferSize,
		inFile:     test.Out.Wav1,
		outFile:    test.Out.Wav2,
		messages:   int(math.Ceil(float64(test.Data.Wav1Samples) / float64(bufferSize))),
		samples:    test.Data.Wav1Samples,
	},
}

func TestWavPipe(t *testing.T) {
	for _, test := range tests {
		pump, err := wav.NewPump(test.inFile, bufferSize)
		assert.Nil(t, err)
		sampleRate := pump.SampleRate()
		sink, err := wav.NewSink(test.outFile, pump.SampleRate(), pump.NumChannels(), pump.BitDepth(), pump.Format())
		assert.Nil(t, err)

		processor := &mock.Processor{}
		p, err := pipe.New(
			sampleRate,
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

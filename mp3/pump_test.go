package mp3_test

import (
	"testing"

	"github.com/pipelined/phono/mp3"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"
	"github.com/stretchr/testify/assert"
)

func TestPump(t *testing.T) {
	bufferSize := 512
	pump, err := mp3.NewPump(test.Data.Mp3, bufferSize)
	assert.Nil(t, err)
	sampleRate := pump.SampleRate()
	sink, err := mp3.NewSink(test.Out.Mp3, pump.SampleRate(), 2, 192, 2)
	assert.Nil(t, err)
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
}

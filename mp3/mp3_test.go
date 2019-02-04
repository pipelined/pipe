package mp3_test

import (
	"testing"

	"github.com/pipelined/phono/mp3"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"
	"github.com/stretchr/testify/assert"
)

func TestMp3(t *testing.T) {
	bufferSize := 512
	pump := mp3.NewPump(test.Data.Mp3)
	sink := mp3.NewSink(test.Out.Mp3, 192, 2)
	p, err := pipe.New(
		bufferSize,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
}

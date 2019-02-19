package mp3_test

import (
	"testing"

	"github.com/pipelined/phono/mp3"
	"github.com/pipelined/phono/pipe"
	"github.com/stretchr/testify/assert"
)

const (
	in  = "../_testdata/sample.mp3"
	out = "../_testdata/out/sample.mp3"
)

func TestMp3(t *testing.T) {
	bufferSize := 512
	pump := mp3.NewPump(in)
	sink := mp3.NewSink(out, 192, 2)
	p, err := pipe.New(
		bufferSize,
		pipe.WithPump(pump),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.Nil(t, err)
	err = pipe.Wait(p.Run())
	assert.NotNil(t, err)
}

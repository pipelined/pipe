package pipe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
)

var (
	bufferSize = 512
)

// TODO: build tests with mock package

func TestPipe(t *testing.T) {

	pump := &mock.Pump{}

	proc := &mock.Processor{}
	sink := &mock.Sink{}

	p := pipe.New(
		pipe.WithPump(pump),
		pipe.WithProcessors(proc),
		pipe.WithSinks(sink),
	)
	err := p.Validate()
	assert.NotNil(t, err)

	proc.Simple = 100
	err = p.Validate()
	assert.Nil(t, err)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	pc := make(chan phono.Options)
	defer close(pc)

	errc, err := p.Run(ctx, pc)
	assert.Nil(t, err)
	err = pipe.WaitPipe(errc)
	assert.Nil(t, err)
}

package repeat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutate"
	"pipelined.dev/pipe/repeat"
)

const bufferSize = 512

func TestAddRoute(t *testing.T) {
	repeater := &repeat.Repeater{
		Mutability: mutate.Mutable(),
	}
	source, _ := pipe.Line{
		Pump: (&mock.Pump{
			Limit:       10 * bufferSize,
			NumChannels: 2,
		}).Pump(),
		Sink: repeater.Sink(),
	}.Route(bufferSize)

	sink1 := &mock.Sink{}
	destination1, _ := pipe.Line{
		Pump: repeater.Pump(),
		Sink: sink1.Sink(),
	}.Route(bufferSize)

	p := pipe.New(
		context.Background(),
		pipe.WithRoutes(source, destination1),
	)
	sink2 := &mock.Sink{}
	line := pipe.Line{
		Sink: sink2.Sink(),
	}
	p.Push(repeater.AddLine(p, line))

	// start
	_ = p.Wait()
	assert.Equal(t, 10, sink1.Counter.Messages)
	assert.Equal(t, 10*bufferSize, sink1.Counter.Samples)
	assert.True(t, sink2.Counter.Messages > 0)
	assert.True(t, sink2.Counter.Samples > 0)
}

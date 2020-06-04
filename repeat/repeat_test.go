package repeat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/repeat"
)

const bufferSize = 512

func TestAddRoute(t *testing.T) {
	repeater := &repeat.Repeater{
		Mutability: mutability.New(),
	}
	source, _ := pipe.Route{
		Pump: (&mock.Pump{
			Limit:    10 * bufferSize,
			Channels: 2,
		}).Pump(),
		Sink: repeater.Sink(),
	}.Line(bufferSize)

	sink1 := &mock.Sink{}
	destination1, _ := pipe.Route{
		Pump: repeater.Pump(),
		Sink: sink1.Sink(),
	}.Line(bufferSize)

	p := pipe.New(
		context.Background(),
		pipe.WithLines(source, destination1),
	)
	sink2 := &mock.Sink{}
	line := pipe.Route{
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

// This benchmark runs the following pipe:
// 1 Pump is repeated to 2 Sinks
func BenchmarkRepeat2(b *testing.B) {
	pump := &mock.Pump{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	repeater := repeat.Repeater{}
	l1, _ := pipe.Route{
		Pump: pump.Pump(),
		Sink: repeater.Sink(),
	}.Line(bufferSize)
	l2, _ := pipe.Route{
		Pump: repeater.Pump(),
		Sink: (&mock.Sink{Discard: true}).Sink(),
	}.Line(bufferSize)
	l3, _ := pipe.Route{
		Pump: repeater.Pump(),
		Sink: (&mock.Sink{Discard: true}).Sink(),
	}.Line(bufferSize)
	for i := 0; i < b.N; i++ {
		p := pipe.New(
			context.Background(),
			pipe.WithLines(l1, l2, l3),
			pipe.WithMutations(pump.Reset()),
		)
		_ = p.Wait()
	}
}

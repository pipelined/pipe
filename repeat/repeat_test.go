package repeat_test

import (
	"context"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/repeat"
)

const bufferSize = 512

func TestAddRoute(t *testing.T) {
	repeater := &repeat.Repeater{
		Mutability: mutability.Mutable(),
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
	assertEqual(t, "sink1 messages", sink1.Counter.Messages, 10)
	assertEqual(t, "sink1 samples", sink1.Counter.Samples, 10*bufferSize)
	assertEqual(t, "sink2 messages", sink2.Counter.Messages > 0, true)
	assertEqual(t, "sink2 samples", sink2.Counter.Samples > 0, true)
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

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

package runtime_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run/internal/runtime"
)

var mockError = errors.New("mock error")

const bufferSize = 512

func TestLines(t *testing.T) {
	type (
		mockLine struct {
			*mock.Source
			*mock.Processor
			*mock.Sink
		}

		assertFunc func(t *testing.T, mocks ...mockLine)
	)

	assertLine := func(t *testing.T, m mockLine, messages, samples int) {
		assertEqual(t, "src messages", m.Source.Counter.Messages, messages)
		assertEqual(t, "proc messages", m.Processor.Counter.Messages, messages)
		assertEqual(t, "sink messages", m.Sink.Counter.Messages, messages)
		assertEqual(t, "src samples", m.Source.Counter.Samples, samples)
		assertEqual(t, "proc samples", m.Processor.Counter.Samples, samples)
		assertEqual(t, "sink samples", m.Sink.Counter.Samples, samples)
	}

	testLines := func(assertFn assertFunc, mocks ...mockLine) func(*testing.T) {
		return func(t *testing.T) {
			routes := make([]pipe.Routing, 0, len(mocks))
			for i := range mocks {
				routes = append(routes, pipe.Routing{
					Source:     mocks[i].Source.Source(),
					Processors: pipe.Processors(mocks[i].Processor.Processor()),
					Sink:       mocks[i].Sink.Sink(),
				})
			}
			p, err := pipe.New(bufferSize, routes...)
			assertEqual(t, "pipe err", err, nil)
			var lines runtime.Lines
			mc := make(chan mutable.Mutations)
			for i := range p.Lines {
				lines.Lines = append(lines.Lines, runtime.LineExecutor(p.Lines[i], mc))
			}
			errChan := lines.Run(context.Background())
			for err := range errChan {
				assertEqual(t, "exec error", err, nil)
			}
			assertFn(t, mocks...)
		}
	}

	t.Run("single line ok", testLines(
		func(t *testing.T, mocks ...mockLine) {
			m := mocks[0]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 3, 1040)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
	t.Run("two lines ok", testLines(
		func(t *testing.T, mocks ...mockLine) {
			var m mockLine
			m = mocks[0]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 3, 1040)
			m = mocks[1]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 4, 1640)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1640,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
	t.Run("three lines ok", testLines(
		func(t *testing.T, mocks ...mockLine) {
			var m mockLine
			m = mocks[0]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 6, 3048)
			m = mocks[1]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 4, 1640)
			m = mocks[2]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 8, 4096)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    3048,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1640,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    4096,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{},
		},
	))
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

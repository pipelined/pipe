package runtime_test

import (
	"context"
	"errors"
	"fmt"
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

		assertFunc func(t *testing.T, err error, mocks ...mockLine)
	)
	mockError := fmt.Errorf("mock error")

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
			errChan := runtime.Run(context.Background(), &lines)
			err = <-errChan
			<-errChan
			assertFn(t, err, mocks...)
		}
	}

	t.Run("two lines processor start error source flush error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			// assertEqual(t, "error", err, mockError)
			m := mocks[0]
			assertEqual(t, "line 1 src started", m.Source.Started, true)
			assertEqual(t, "line 1 proc started", m.Processor.Started, true)
			assertEqual(t, "line 1 sink started", m.Sink.Started, true)
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			m = mocks[1]
			assertEqual(t, "line 2 src started", m.Source.Started, true)
			assertEqual(t, "line 2 proc started", m.Processor.Started, true)
			assertEqual(t, "line 2 sink started", m.Sink.Started, false)
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
				Flusher: mock.Flusher{
					ErrorOnFlush: mockError,
				},
			},
			Processor: &mock.Processor{},
			Sink:      &mock.Sink{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("two lines processor start error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			// assertEqual(t, "error", err, mockError)
			m := mocks[0]
			assertEqual(t, "src started", m.Source.Started, true)
			assertEqual(t, "proc started", m.Processor.Started, true)
			assertEqual(t, "sink started", m.Sink.Started, true)
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
			m = mocks[1]
			assertEqual(t, "line 2 src started", m.Source.Started, true)
			assertEqual(t, "line 2 proc started", m.Processor.Started, true)
			assertEqual(t, "line 2 sink started", m.Sink.Started, false)
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{},
			Sink:      &mock.Sink{},
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("single line processor start error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			// assertEqual(t, "error", errors.Is(err, mockError), true)
			m := mocks[0]
			assertEqual(t, "line 1 src started", m.Source.Started, true)
			assertEqual(t, "line 1 proc started", m.Processor.Started, true)
			assertEqual(t, "line 1 sink started", m.Sink.Started, false)
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, false)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, false)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Processor: &mock.Processor{
				Starter: mock.Starter{
					ErrorOnStart: mockError,
				},
			},
			Sink: &mock.Sink{},
		},
	))
	t.Run("single line ok", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			m := mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
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
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			var m mockLine
			m = mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 3, 1040)
			m = mocks[1]
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, true)
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
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "exec error", err, nil)
			var m mockLine
			m = mocks[0]
			assertEqual(t, "line 1 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 1 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 1 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 6, 3048)
			m = mocks[1]
			assertEqual(t, "line 2 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 2 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 2 sink flushed", m.Sink.Flushed, true)
			assertLine(t, m, 4, 1640)
			m = mocks[2]
			assertEqual(t, "line 3 src flushed", m.Source.Flushed, true)
			assertEqual(t, "line 3 proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "line 3 sink flushed", m.Sink.Flushed, true)
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
	t.Run("single processor error", testLines(
		func(t *testing.T, err error, mocks ...mockLine) {
			assertEqual(t, "error", errors.Is(err, mockError), true)
			m := mocks[0]
			assertEqual(t, "src flushed", m.Source.Flushed, true)
			assertEqual(t, "proc flushed", m.Processor.Flushed, true)
			assertEqual(t, "sink flushed", m.Sink.Flushed, true)
		},
		mockLine{
			Source: &mock.Source{
				Limit:    1040,
				Channels: 1,
			},
			Sink: &mock.Sink{
				Discard: true,
			},
			Processor: &mock.Processor{
				ErrorOnCall: mockError,
			},
		},
	))

}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

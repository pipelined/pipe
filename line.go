package pipe

import (
	"context"
	"io"

	"pipelined.dev/pipe/internal/async"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type lineRunner struct {
	mutations chan mutable.Mutations
	// mctx      mutable.Context
	Source
	Processors []Processor
	Sink
}

type linesRunner struct {
	bufferSize int
	lines      []lineRunner
}

func (lr linesRunner) Run(ctx context.Context) <-chan error {
	// todo: determine buffer size
	errc := make(chan error, 1)
	go lr.run(ctx, errc)
	return errc
}

// TODO: handle context
// TODO: handle mutations
// TODO: handle errors
func (lr linesRunner) run(ctx context.Context, errc chan error) {
	defer close(errc)

	// start fn
	for _, l := range lr.lines {
		err := callHook(ctx, l.Source.StartFunc)
		if err != nil {
			panic("handle this")
		}

		for j := range l.Processors {
			err := callHook(ctx, l.Processors[j].StartFunc)
			if err != nil {
				panic("handle this")
			}
		}

		err = callHook(ctx, l.Sink.StartFunc)
		if err != nil {
			panic("handle this")
		}
	}

	pools := make(map[int]*signal.PoolAllocator)
	// get all pooled allocators
	for _, l := range lr.lines {
		pools[l.Source.Output.Channels] = signal.GetPoolAllocator(l.Source.Output.Channels, lr.bufferSize, lr.bufferSize)
		for j := range l.Processors {
			pools[l.Processors[j].Output.Channels] = signal.GetPoolAllocator(l.Processors[j].Output.Channels, lr.bufferSize, lr.bufferSize)
		}
		pools[l.Sink.Output.Channels] = signal.GetPoolAllocator(l.Sink.Output.Channels, lr.bufferSize, lr.bufferSize)
	}

	var in, out signal.Floating
	// TODO: add break condition
processing:
	for {
		for _, l := range lr.lines {
			pool := pools[l.Source.Output.Channels]
			out = pool.GetFloat64()

			n, err := l.Source.SourceFunc(out)
			if err != nil {
				if err != io.EOF {
					errc <- err
				}
				l.flush(ctx, errc)
				break processing // TODO:  flush other lines
			}
			if n != out.Length() {
				out = out.Slice(0, n)
			}

			in = out
			for i := range l.Processors {
				outPool := pools[l.Processors[i].Output.Channels]
				out = outPool.GetFloat64()

				err := l.Processors[i].ProcessFunc(in, out)
				if err != nil {
					l.flush(ctx, errc)
					break processing
				}

				in.Free(pool)
				pool = outPool
				in = out
			}

			err = l.SinkFunc(in)
			if err != nil {
				l.flush(ctx, errc)
				break processing
			}
			in.Free(pool)
		}
	}
}

func (lr lineRunner) flush(ctx context.Context, errc chan error) {
	err := callHook(ctx, lr.Source.FlushFunc)
	if err != nil {
		errc <- err
	}

	for i := range lr.Processors {
		err = callHook(ctx, lr.Processors[i].FlushFunc)
		if err != nil {
			errc <- err
		}
	}
	err = callHook(ctx, lr.Sink.FlushFunc)
	if err != nil {
		errc <- err
	}
}

func (lr linesRunner) Out() <-chan async.Message {
	return nil
}

func (lr linesRunner) OutputPool() *signal.PoolAllocator {
	return nil
}

func (lr linesRunner) Insert(async.Runner, mutable.MutatorFunc) mutable.Mutation {
	return mutable.Mutation{}
}

func callHook(ctx context.Context, hook func(context.Context) error) error {
	if hook != nil {
		return hook(ctx)
	}
	return nil
}

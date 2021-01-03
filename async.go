package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/pipe/internal/async"
	"pipelined.dev/pipe/mutable"
)

type (
	// Async executes the pipe asynchronously. Every pipe component is
	// executed in its own goroutine.
	Async struct {
		ctx           context.Context
		mutCtx        mutable.Context
		execCtxs      map[mutable.Context]executionContext
		cancelFn      context.CancelFunc
		merger        *errorMerger
		mutationsChan chan []mutable.Mutation
		errorChan     chan error
	}

	// exectutionContext is a runner of component with a channel which is
	// used as a source of mutations for the component.
	executionContext struct {
		runner    async.Runner
		mutations chan mutable.Mutations
	}

	mutationsCache map[chan mutable.Mutations]mutable.Mutations
)

func newMutationsCache(execCtxs map[mutable.Context]executionContext, initializers []mutable.Mutation) mutationsCache {
	mutCache := make(map[chan mutable.Mutations]mutable.Mutations)
	for i := range initializers {
		if c, ok := execCtxs[initializers[i].Context]; ok {
			mutCache[c.mutations] = mutCache[c.mutations].Put(initializers[i])
		}
	}
	return mutCache
}

// Async creates and starts new pipe.
func (p *Pipe) Async(ctx context.Context, initializers ...mutable.Mutation) *Async {
	// cancel is required to stop the pipe in case of error
	ctx, cancelFn := context.WithCancel(ctx)

	// start the pipe execution with new context
	a := Async{
		ctx:           ctx,
		mutCtx:        mutable.Mutable(),
		execCtxs:      make(map[mutable.Context]executionContext),
		cancelFn:      cancelFn,
		mutationsChan: make(chan []mutable.Mutation, 1),
		errorChan:     make(chan error, 1),
		merger: &errorMerger{
			errorChan: make(chan error, 1),
		},
	}
	for i := range p.Lines {
		a.bindLine(p.Lines[i])
	}
	// push initializers before start
	mutCache := newMutationsCache(a.execCtxs, initializers)
	mutCache.push(ctx)
	a.merger.add(a.startAll()...)
	go a.merger.wait()
	go a.start(mutCache)
	return &a
}

func (a *Async) start(mc mutationsCache) {
	defer close(a.errorChan)
	for {
		select {
		case ms := <-a.mutationsChan:
			for _, m := range ms {
				// mutate the runner itself
				if m.Context == a.mutCtx {
					m.Apply()
				} else {
					if c, ok := a.execCtxs[m.Context]; ok {
						mc[c.mutations] = mc[c.mutations].Put(m)
					} else {
						panic("no listener found!")
					}
				}
			}
			mc.push(a.ctx)
		case err, ok := <-a.merger.errorChan:
			// merger has buffer of one error, if more errors happen, they
			// will be ignored.
			if ok {
				a.cancelFn()
				a.merger.drain()
				a.errorChan <- err
			}
			return
		}
	}
}

// Line binds components. All allocators are executed and wrapped into
// executionContext. If any of allocators failed, the error will be
// returned and flush hooks won't be triggered.
func (a *Async) bindLine(l *Line) {
	mc := make(chan mutable.Mutations, 1)
	// TODO sync context
	var r async.Runner
	r = async.Source(
		mc,
		l.Source.mctx,
		l.Source.Output.poolAllocator(l.bufferSize),
		async.SourceFunc(l.Source.SourceFunc),
		async.HookFunc(l.Source.StartFunc),
		async.HookFunc(l.Source.FlushFunc),
	)
	a.execCtxs[l.Source.mctx] = executionContext{
		mutations: mc,
		runner:    r,
	}

	in := r.Out()
	props := l.Source.Output
	for i := range l.Processors {
		r = async.Processor(
			l.Processors[i].mctx,
			in,
			props.poolAllocator(l.bufferSize),
			l.Processors[i].Output.poolAllocator(l.bufferSize),
			async.ProcessFunc(l.Processors[i].ProcessFunc),
			async.HookFunc(l.Processors[i].StartFunc),
			async.HookFunc(l.Processors[i].FlushFunc),
		)
		a.execCtxs[l.Processors[i].mctx] = executionContext{
			mutations: mc,
			runner:    r,
		}
		in = r.Out()
		props = l.Processors[i].Output
	}

	a.execCtxs[l.Sink.mctx] = executionContext{
		mutations: mc,
		runner: async.Sink(
			l.Sink.mctx,
			in,
			props.poolAllocator(l.bufferSize),
			async.SinkFunc(l.Sink.SinkFunc),
			async.HookFunc(l.Sink.StartFunc),
			async.HookFunc(l.Sink.FlushFunc),
		),
	}
}

func (mc mutationsCache) push(ctx context.Context) {
	for c, m := range mc {
		select {
		case c <- m:
			delete(mc, c)
		case <-ctx.Done():
			return
		}
	}
}

// start starts the execution of pipe.
func (a *Async) startAll() []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, len(a.execCtxs))
	for i := range a.execCtxs {
		errChans = append(errChans, a.execCtxs[i].runner.Run(a.ctx))
	}
	return errChans
}

// Push new mutators into pipe. Calling this method after pipe is done will
// cause a panic.
func (a *Async) Push(mutations ...mutable.Mutation) {
	a.mutationsChan <- mutations
}

// StartLine adds the line to the running pipe. The result is a channel
// that will be closed when line is started or the async execution is done.
// Line should not be mutated while the returned channel is open.
func (a *Async) StartLine(l *Line) <-chan struct{} {
	startCtx, cancelFn := context.WithCancel(a.ctx)
	a.Push(a.startLineMut(l, cancelFn))
	return startCtx.Done()
}

func (a *Async) startLineMut(l *Line, cancelFn context.CancelFunc) mutable.Mutation {
	// TODO sync execution
	return a.mutCtx.Mutate(func() {
		a.bindLine(l)
		a.merger.add(a.startLine(a.ctx, l)...)
		cancelFn()
	})
}

// startLine starts the execution of line.
func (a *Async) startLine(ctx context.Context, l *Line) []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, l.numRunners())
	errChans = append(errChans, a.execCtxs[l.Source.mctx].runner.Run(ctx))
	for i := range l.Processors {
		errChans = append(errChans, a.execCtxs[l.Processors[i].mctx].runner.Run(ctx))
	}
	errChans = append(errChans, a.execCtxs[l.Sink.mctx].runner.Run(ctx))
	return errChans
}

// StartProcessor adds the processor to the running line. Pos is the index
// of the processor that should be started. The result is a channel that
// will be closed when processor is started or the async exection is done.
// No other processors should be starting in this line while the returned
// channel is open.
func (a *Async) StartProcessor(l *Line, pos int) <-chan struct{} {
	if _, ok := a.execCtxs[l.Source.mctx]; !ok {
		panic("line is not running")
	}
	proc := l.Processors[pos]
	if _, ok := a.execCtxs[proc.mctx]; ok {
		panic(fmt.Sprintf("processor at %d position is already running", pos))
	}
	prev := a.execCtxs[l.prev(pos)]
	next := a.execCtxs[l.next(pos)]

	r := async.Processor(
		proc.mctx,
		prev.runner.Out(),
		prev.runner.OutputPool(),
		proc.Output.poolAllocator(l.bufferSize),
		async.ProcessFunc(proc.ProcessFunc),
		async.HookFunc(proc.StartFunc),
		async.HookFunc(proc.FlushFunc),
	)
	a.execCtxs[proc.mctx] = executionContext{
		mutations: prev.mutations,
		runner:    r,
	}
	ctx, cancelFn := context.WithCancel(a.ctx)
	a.Push(
		next.runner.Insert(r, a.startRunnerMutFunc(r, proc.mctx, cancelFn)),
	)
	return ctx.Done()
}

func (a *Async) startRunnerMutFunc(r async.Runner, mctx mutable.Context, cancelFn context.CancelFunc) mutable.MutatorFunc {
	return func() {
		a.merger.add(r.Run(a.ctx))
		cancelFn()
	}
}

// Await for successful finish or first error to occur.
func (a *Async) Await() error {
	for err := range a.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

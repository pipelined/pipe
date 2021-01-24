package run

import (
	"context"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run/internal/runtime"
	"pipelined.dev/signal"
)

type (
	// Run executes the pipe asynchronously.
	Run struct {
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
		executor  runtime.Executor
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

// New creates and starts new pipe.
func New(ctx context.Context, p *pipe.Pipe, initializers ...mutable.Mutation) *Run {
	// cancel is required to stop the pipe in case of error
	ctx, cancelFn := context.WithCancel(ctx)

	// start the pipe execution with new context
	a := Run{
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

func (a *Run) start(mc mutationsCache) {
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
func (a *Run) bindLine(l *pipe.Line) {
	mc := make(chan mutable.Mutations, 1)
	if l.Context.IsMutable() {
		a.bindSync(l, mc)
		return
	}
	a.bindAsync(l, mc)
}

func (a *Run) bindSync(l *pipe.Line, mc chan mutable.Mutations) {
	line := runtime.LineExecutor(l, mc)

	if e, ok := a.execCtxs[l.Context]; !ok {
		a.execCtxs[l.Context] = executionContext{
			mutations: mc,
			executor:  &runtime.Lines{Lines: []runtime.Line{line}},
		}
	} else {
		if ex, ok := e.executor.(*runtime.Lines); ok {
			ex.Lines = append(ex.Lines, line)
		} else {
			panic("add executors to component context")
		}
	}
}

func (a *Run) bindAsync(l *pipe.Line, mc chan mutable.Mutations) {
	var (
		sender, receiver runtime.Link
		input, output    *signal.PoolAllocator
	)
	sender = runtime.AsyncLink()
	output = l.SourceOutputPool()
	a.execCtxs[l.Source.Context] = executionContext{
		mutations: mc,
		executor:  runtime.SourceExecutor(l.Source, mc, output, sender),
	}
	for i := range l.Processors {
		receiver, sender = sender, runtime.AsyncLink()
		input, output = output, l.ProcessorOutputPool(i)
		a.execCtxs[l.Processors[i].Context] = executionContext{
			mutations: mc,
			executor:  runtime.ProcessExecutor(l.Processors[i], input, output, receiver, sender),
		}
	}
	a.execCtxs[l.Sink.Context] = executionContext{
		mutations: mc,
		executor:  runtime.SinkExecutor(l.Sink, output, sender),
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
func (a *Run) startAll() []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, len(a.execCtxs))
	for i := range a.execCtxs {
		errChans = append(errChans, runtime.Start(a.ctx, a.execCtxs[i].executor))
	}
	return errChans
}

// Push new mutators into pipe. Calling this method after pipe is done will
// cause a panic.
func (a *Run) Push(mutations ...mutable.Mutation) {
	a.mutationsChan <- mutations
}

// StartLine adds the line to the running pipe. The result is a channel
// that will be closed when line is started or the async execution is done.
// Line should not be mutated while the returned channel is open.
func (a *Run) StartLine(l *pipe.Line) <-chan struct{} {
	startCtx, cancelFn := context.WithCancel(a.ctx)
	a.Push(a.startLineMut(l, cancelFn))
	return startCtx.Done()
}

func (a *Run) startLineMut(l *pipe.Line, cancelFn context.CancelFunc) mutable.Mutation {
	// TODO sync execution
	return a.mutCtx.Mutate(func() {
		a.bindLine(l)
		a.merger.add(a.startLine(a.ctx, l)...)
		cancelFn()
	})
}

// startLine starts the execution of line.
func (a *Run) startLine(ctx context.Context, l *pipe.Line) []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, numRunners(l))
	errChans = append(errChans, runtime.Start(a.ctx, a.execCtxs[l.Source.Context].executor))
	for i := range l.Processors {
		errChans = append(errChans, runtime.Start(a.ctx, a.execCtxs[l.Processors[i].Context].executor))
	}
	errChans = append(errChans, runtime.Start(a.ctx, a.execCtxs[l.Sink.Context].executor))
	return errChans
}

// StartProcessor adds the processor to the running line. Pos is the index
// of the processor that should be started. The result is a channel that
// will be closed when processor is started or the async exection is done.
// No other processors should be starting in this line while the returned
// channel is open.
// func (a *Async) StartProcessor(l *Line, pos int) <-chan struct{} {
// 	if _, ok := a.execCtxs[l.Source.mctx]; !ok {
// 		panic("line is not running")
// 	}
// 	proc := l.Processors[pos]
// 	if _, ok := a.execCtxs[proc.mctx]; ok {
// 		panic(fmt.Sprintf("processor at %d position is already running", pos))
// 	}
// 	prev := a.execCtxs[l.prev(pos)]
// 	next := a.execCtxs[l.next(pos)]

// 	r := runtime.Processor(
// 		proc.mctx,
// 		prev.runner.Out(),
// 		prev.runner.OutputPool(),
// 		proc.Output.poolAllocator(l.bufferSize),
// 		runtime.ProcessFunc(proc.ProcessFunc),
// 		runtime.HookFunc(proc.StartFunc),
// 		runtime.HookFunc(proc.FlushFunc),
// 	)
// 	a.execCtxs[proc.mctx] = executionContext{
// 		mutations: prev.mutations,
// 		runner:    r,
// 	}
// 	ctx, cancelFn := context.WithCancel(a.ctx)
// 	a.Push(
// 		next.runner.Insert(r, a.startRunnerMutFunc(r, proc.mctx, cancelFn)),
// 	)
// 	return ctx.Done()
// }

// func (a *Runner) startRunnerMutFunc(r Starter, mctx mutable.Context, cancelFn context.CancelFunc) mutable.MutatorFunc {
// 	return func() {
// 		a.merger.add(r.Start(a.ctx))
// 		cancelFn()
// 	}
// }

// Wait for successful finish or first error to occur.
func (a *Run) Wait() error {
	for err := range a.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

func numRunners(l *pipe.Line) int {
	return 2 + len(l.Processors)
}

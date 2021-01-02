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

	executionContext struct {
		runner    async.Runner
		mutations chan mutable.Mutations
	}
)

// Async creates and starts new pipe.
func (p *Pipe) Async(ctx context.Context, initializers ...mutable.Mutation) *Async {
	ctx, cancelFn := context.WithCancel(ctx)
	execCtxs := p.bind()
	mutations := make(map[chan mutable.Mutations]mutable.Mutations)
	for i := range initializers {
		if c, ok := execCtxs[initializers[i].Context]; ok {
			mutations[c.mutations] = mutations[c.mutations].Put(initializers[i])
		}
	}
	// push cached mutators at the start
	push(ctx, mutations)
	// start the pipe execution with new context
	// cancel is required to stop the pipe in case of error
	merger := errorMerger{
		errorChan: make(chan error, 1),
	}
	merger.add(startAll(ctx, execCtxs)...)
	go merger.start()

	errc := make(chan error, 1)
	mutationsChan := make(chan []mutable.Mutation, 1)
	pipeMutCtx := mutable.Mutable()
	go func() {
		defer close(errc)
		for {
			select {
			case ms := <-mutationsChan:
				for _, m := range ms {
					// mutate the runner itself
					if m.Context == pipeMutCtx {
						m.Apply()
					} else {
						if c, ok := execCtxs[m.Context]; ok {
							mutations[c.mutations] = mutations[c.mutations].Put(m)
						} else {
							panic("no listener found!")
						}
					}
				}
				push(ctx, mutations)
			case err, ok := <-merger.errorChan:
				// merger has buffer of one error,
				// if more errors happen, they will be ignored.
				if ok {
					cancelFn()
					merger.await()
					errc <- err
				}
				return
			}
		}
	}()
	return &Async{
		ctx:           ctx,
		mutCtx:        pipeMutCtx,
		cancelFn:      cancelFn,
		mutationsChan: mutationsChan,
		errorChan:     errc,
		merger:        &merger,
		execCtxs:      execCtxs,
	}
}

func (p *Pipe) bind() map[mutable.Context]executionContext {
	ectxs := make(map[mutable.Context]executionContext)
	for _, l := range p.Lines {
		// TODO sync context
		// if l.mctx.IsMutable() {
		// 	continue
		// }

		l.asyncExecution(ectxs)
	}
	return ectxs
}

// Line binds components. All allocators are executed and wrapped into
// executionContext. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (l *Line) asyncExecution(exCtxs map[mutable.Context]executionContext) {
	mc := make(chan mutable.Mutations, 1)
	var r async.Runner
	r = async.Source(
		mc,
		l.Source.mctx,
		l.Source.Output.poolAllocator(l.bufferSize),
		async.SourceFunc(l.Source.SourceFunc),
		async.HookFunc(l.Source.StartFunc),
		async.HookFunc(l.Source.FlushFunc),
	)
	exCtxs[l.Source.mctx] = executionContext{
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
		exCtxs[l.Processors[i].mctx] = executionContext{
			mutations: mc,
			runner:    r,
		}
		in = r.Out()
		props = l.Processors[i].Output
	}

	exCtxs[l.Sink.mctx] = executionContext{
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

func (l *Line) mutableContexts() []mutable.Context {
	ctxs := make([]mutable.Context, 0, 2+len(l.Processors))

	ctxs = append(ctxs, l.Source.mctx, l.Sink.mctx)
	for i := range l.Processors {
		ctxs = append(ctxs, l.Processors[i].mctx)
	}

	return ctxs
}

func push(ctx context.Context, mutations map[chan mutable.Mutations]mutable.Mutations) {
	for c, m := range mutations {
		select {
		case c <- m:
			delete(mutations, c)
		case <-ctx.Done():
			return
		}
	}
}

// start starts the execution of pipe.
func startAll(ctx context.Context, executors map[mutable.Context]executionContext) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, len(executors))
	for i := range executors {
		errChans = append(errChans, executors[i].runner.Run(ctx))
	}
	return errChans
}

// start starts the execution of pipe.
// TODO: rewrite
func start(ctx context.Context, runners map[mutable.Context]executionContext, contexts []mutable.Context) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, len(contexts))
	for i := range contexts {
		errChans = append(errChans, runners[contexts[i]].runner.Run(ctx))
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
	ctx, cancelFn := context.WithCancel(a.ctx)
	a.Push(a.startLineMut(l, cancelFn))
	return ctx.Done()
}

func (a *Async) startLineMut(l *Line, cancelFn context.CancelFunc) mutable.Mutation {
	// TODO sync execution
	return a.mutCtx.Mutate(func() {
		l.asyncExecution(a.execCtxs)
		ctxs := l.mutableContexts()
		a.merger.add(start(a.ctx, a.execCtxs, ctxs)...)
		cancelFn()
	})
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
		a.merger.add(start(a.ctx, map[mutable.Context]executionContext{mctx: {runner: r}}, []mutable.Context{mctx})...)
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

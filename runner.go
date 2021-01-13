package pipe

import (
	"context"

	"pipelined.dev/pipe/internal/async"
	"pipelined.dev/pipe/internal/execution"
	"pipelined.dev/pipe/mutable"
)

type (
	// Runner executes the pipe asynchronously. Every pipe component is
	// executed in its own goroutine.
	Runner struct {
		ctx           context.Context
		mutCtx        mutable.Context
		execCtxs      map[mutable.Context]executionContext
		cancelFn      context.CancelFunc
		merger        *errorMerger
		mutationsChan chan []mutable.Mutation
		errorChan     chan error
	}

	// Starter is asynchronous component executor.
	Starter interface {
		Start(context.Context) <-chan error
		// Out() <-chan Message
		// OutputPool() *signal.PoolAllocator
		// Insert(Runner, mutable.MutatorFunc) mutable.Mutation
	}

	// exectutionContext is a runner of component with a channel which is
	// used as a source of mutations for the component.
	executionContext struct {
		starter   Starter
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

// Run creates and starts new pipe.
func (p *Pipe) Run(ctx context.Context, initializers ...mutable.Mutation) *Runner {
	// cancel is required to stop the pipe in case of error
	ctx, cancelFn := context.WithCancel(ctx)

	// start the pipe execution with new context
	a := Runner{
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

func (a *Runner) start(mc mutationsCache) {
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
func (a *Runner) bindLine(l *Line) {
	mc := make(chan mutable.Mutations, 1)
	if l.mctx.IsMutable() {
		a.bindSync(l, mc)
		return
	}
	a.bindAsync(l, mc)
	// TODO sync context
}

func (a *Runner) bindSync(l *Line, mc chan mutable.Mutations) {
	executors := make([]async.Executor, 0, 2+len(l.Processors))
	sender := execution.SyncLink()
	executors = append(executors, l.Source.Executor(mc, l.bufferSize, sender))

	inputProps := l.Source.Output
	receiver, sender := sender, execution.AsyncLink()
	for i := range l.Processors {
		executors = append(executors, l.Processors[i].Executor(l.bufferSize, inputProps, receiver, sender))
	}
	executors = append(executors, l.Sink.Executor(l.bufferSize, inputProps, receiver))

	if e, ok := a.execCtxs[l.mctx]; ok {
		_ = e.starter.(*async.LineStarter)
		// TODO: add
	} else {
		a.execCtxs[l.mctx] = executionContext{
			mutations: mc,
			starter: &async.LineStarter{
				Executors: []async.Executor{&execution.Line{Executors: executors}},
			},
		}
	}
	return
}

func (a *Runner) bindAsync(l *Line, mc chan mutable.Mutations) {
	sender := execution.AsyncLink()
	a.execCtxs[l.Source.mctx] = executionContext{
		mutations: mc,
		starter:   &async.ComponentStarter{Executor: l.Source.Executor(mc, l.bufferSize, sender)},
	}
	inputProps := l.Source.Output
	receiver, sender := sender, execution.AsyncLink()
	for i := range l.Processors {
		a.execCtxs[l.Processors[i].mctx] = executionContext{
			mutations: mc,
			starter:   &async.ComponentStarter{Executor: l.Processors[i].Executor(l.bufferSize, inputProps, receiver, sender)},
		}
		inputProps = l.Processors[i].Output
		receiver, sender = sender, execution.AsyncLink()
	}
	a.execCtxs[l.Sink.mctx] = executionContext{
		mutations: mc,
		starter:   &async.ComponentStarter{Executor: l.Sink.Executor(l.bufferSize, inputProps, receiver)},
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
func (a *Runner) startAll() []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, len(a.execCtxs))
	for i := range a.execCtxs {
		errChans = append(errChans, a.execCtxs[i].starter.Start(a.ctx))
	}
	return errChans
}

// Push new mutators into pipe. Calling this method after pipe is done will
// cause a panic.
func (a *Runner) Push(mutations ...mutable.Mutation) {
	a.mutationsChan <- mutations
}

// StartLine adds the line to the running pipe. The result is a channel
// that will be closed when line is started or the async execution is done.
// Line should not be mutated while the returned channel is open.
func (a *Runner) StartLine(l *Line) <-chan struct{} {
	startCtx, cancelFn := context.WithCancel(a.ctx)
	a.Push(a.startLineMut(l, cancelFn))
	return startCtx.Done()
}

func (a *Runner) startLineMut(l *Line, cancelFn context.CancelFunc) mutable.Mutation {
	// TODO sync execution
	return a.mutCtx.Mutate(func() {
		a.bindLine(l)
		a.merger.add(a.startLine(a.ctx, l)...)
		cancelFn()
	})
}

// startLine starts the execution of line.
func (a *Runner) startLine(ctx context.Context, l *Line) []<-chan error {
	// start all runners error channel for each component
	errChans := make([]<-chan error, 0, l.numRunners())
	errChans = append(errChans, a.execCtxs[l.Source.mctx].starter.Start(ctx))
	for i := range l.Processors {
		errChans = append(errChans, a.execCtxs[l.Processors[i].mctx].starter.Start(ctx))
	}
	errChans = append(errChans, a.execCtxs[l.Sink.mctx].starter.Start(ctx))
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

// 	r := async.Processor(
// 		proc.mctx,
// 		prev.runner.Out(),
// 		prev.runner.OutputPool(),
// 		proc.Output.poolAllocator(l.bufferSize),
// 		async.ProcessFunc(proc.ProcessFunc),
// 		async.HookFunc(proc.StartFunc),
// 		async.HookFunc(proc.FlushFunc),
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

func (a *Runner) startRunnerMutFunc(r Starter, mctx mutable.Context, cancelFn context.CancelFunc) mutable.MutatorFunc {
	return func() {
		a.merger.add(r.Start(a.ctx))
		cancelFn()
	}
}

// Wait for successful finish or first error to occur.
func (a *Runner) Wait() error {
	for err := range a.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// Executor returns executor for source component.
func (s Source) Executor(mc chan mutable.Mutations, bufferSize int, sender execution.Link) async.Executor {
	return execution.Source{
		Mutations:  mc,
		Context:    s.mctx,
		OutputPool: s.Output.poolAllocator(bufferSize),
		SourceFn:   execution.SourceFunc(s.SourceFunc),
		StartFunc:  execution.StartFunc(s.StartFunc),
		FlushFunc:  execution.FlushFunc(s.FlushFunc),
		Sender:     sender,
	}
}

// Executor returns executor for processor component.
func (p Processor) Executor(bufferSize int, input SignalProperties, receiver, sender execution.Link) async.Executor {
	return execution.Processor{
		Context:    p.mctx,
		InputPool:  input.poolAllocator(bufferSize),
		OutputPool: p.Output.poolAllocator(bufferSize),
		ProcessFn:  execution.ProcessFunc(p.ProcessFunc),
		StartFunc:  execution.StartFunc(p.StartFunc),
		FlushFunc:  execution.FlushFunc(p.FlushFunc),
		Receiver:   receiver,
		Sender:     sender,
	}
}

// Executor returns executor for processor component.
func (s Sink) Executor(bufferSize int, input SignalProperties, receiver execution.Link) async.Executor {
	return execution.Sink{
		Context:   s.mctx,
		InputPool: input.poolAllocator(bufferSize),
		SinkFn:    execution.SinkFunc(s.SinkFunc),
		StartFunc: execution.StartFunc(s.StartFunc),
		FlushFunc: execution.FlushFunc(s.FlushFunc),
		Receiver:  receiver,
	}
}

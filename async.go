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
		cancelFn      context.CancelFunc
		mctx          mutable.Context
		merger        *errorMerger
		listeners     map[mutable.Context]chan mutable.Mutations
		runners       map[mutable.Context]async.Runner
		mutations     map[chan mutable.Mutations]mutable.Mutations
		mutationsChan chan []mutable.Mutation
		errorChan     chan error
	}
)

// Async creates and starts new pipe.
func (p *Pipe) Async(ctx context.Context, initializers ...mutable.Mutation) *Async {
	ctx, cancelFn := context.WithCancel(ctx)
	runners := make(map[mutable.Context]async.Runner)
	listeners := make(map[mutable.Context]chan mutable.Mutations)
	for i := range p.Lines {
		mutationsChan := make(chan mutable.Mutations, 1)
		p.Lines[i].runners(ctx, mutationsChan, runners)
		addListeners(listeners,
			mutationsChan,
			p.Lines[i].mutableContexts()...,
		)
	}
	mutations := make(map[chan mutable.Mutations]mutable.Mutations)
	for i := range initializers {
		if c := listeners[initializers[i].Context]; c != nil {
			mutations[c] = mutations[c].Put(initializers[i])
		}
	}
	// push cached mutators at the start
	push(mutations)
	// start the pipe execution with new context
	// cancel is required to stop the pipe in case of error
	merger := errorMerger{
		errorChan: make(chan error, 1),
	}
	merger.add(startAll(ctx, runners)...)
	go merger.start()

	errc := make(chan error, 1)
	mutationsChan := make(chan []mutable.Mutation, 1)
	runnerContext := mutable.Mutable()
	go func() {
		defer close(errc)
		for {
			select {
			case ms := <-mutationsChan:
				for _, m := range ms {
					// mutate the runner itself
					if m.Context == runnerContext {
						if err := m.Apply(); err != nil {
							cancelFn()
							merger.await()
							errc <- err
						}
					} else {
						if c, ok := listeners[m.Context]; ok {
							mutations[c] = mutations[c].Put(m)
						} else {
							panic("no listener found!")
						}
					}
				}
				push(mutations)
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
		cancelFn:      cancelFn,
		mctx:          runnerContext,
		mutations:     mutations,
		mutationsChan: mutationsChan,
		errorChan:     errc,
		merger:        &merger,
		listeners:     listeners,
		runners:       runners,
	}
}

// Line binds components. All allocators are executed and wrapped into
// runners. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (l *Line) runners(ctx context.Context, mc chan mutable.Mutations, runners map[mutable.Context]async.Runner) {
	var r async.Runner
	r = async.Source(
		mc,
		l.Source.mctx,
		l.Source.Output.poolAllocator(l.bufferSize),
		async.SourceFunc(l.Source.SourceFunc),
		async.HookFunc(l.Source.StartFunc),
		async.HookFunc(l.Source.FlushFunc),
	)
	runners[l.Source.mctx] = r

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
		runners[l.Processors[i].mctx] = r
		in = r.Out()
		props = l.Processors[i].Output
	}

	runners[l.Sink.mctx] = async.Sink(
		l.Sink.mctx,
		in,
		props.poolAllocator(l.bufferSize),
		async.SinkFunc(l.Sink.SinkFunc),
		async.HookFunc(l.Sink.StartFunc),
		async.HookFunc(l.Sink.FlushFunc),
	)
}

func (l *Line) mutableContexts() []mutable.Context {
	ctxs := make([]mutable.Context, 0, 2+len(l.Processors))

	ctxs = append(ctxs, l.Source.mctx, l.Sink.mctx)
	for i := range l.Processors {
		ctxs = append(ctxs, l.Processors[i].mctx)
	}

	return ctxs
}

func push(mutations map[chan mutable.Mutations]mutable.Mutations) {
	for c, m := range mutations {
		c <- m
		delete(mutations, c)
	}
}

// start starts the execution of pipe.
func startAll(ctx context.Context, runners map[mutable.Context]async.Runner) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, len(runners))
	for i := range runners {
		errChans = append(errChans, runners[i].Run(ctx))
	}
	return errChans
}

// start starts the execution of pipe.
func start(ctx context.Context, runners map[mutable.Context]async.Runner, contexts []mutable.Context) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, len(contexts))
	for i := range contexts {
		errChans = append(errChans, runners[contexts[i]].Run(ctx))
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
	mc := make(chan mutable.Mutations, 1)
	l.runners(a.ctx, mc, a.runners)
	ctxs := l.mutableContexts()
	ctx, cancelFn := context.WithCancel(a.ctx)
	a.Push(
		a.mctx.Mutate(func() error {
			addListeners(a.listeners, mc, ctxs...)
			a.merger.add(start(a.ctx, a.runners, ctxs)...)
			cancelFn()
			return nil
		}),
	)
	return ctx.Done()
}

// StartProcessor adds the processor to the running line. Pos is the index
// of the processor that should be started. The result is a channel that
// will be closed when processor is started or the async exection is done.
// No other processors should be starting in this line while the returned
// channel is open.
func (a *Async) StartProcessor(l *Line, pos int) <-chan struct{} {
	proc := l.Processors[pos]
	if _, ok := a.runners[proc.mctx]; ok {
		panic(fmt.Sprintf("processor at %d position is already running", pos))
	}
	prev := a.runners[l.prev(pos)]
	next := a.runners[l.next(pos)]

	r := async.Processor(
		proc.mctx,
		prev.Out(),
		prev.OutputPool(),
		proc.Output.poolAllocator(l.bufferSize),
		async.ProcessFunc(proc.ProcessFunc),
		async.HookFunc(proc.StartFunc),
		async.HookFunc(proc.FlushFunc),
	)
	a.runners[proc.mctx] = r
	ctx, cancelFn := context.WithCancel(a.ctx)
	a.Push(
		a.mctx.Mutate(func() error {
			addListeners(a.listeners, a.listeners[l.Source.mctx], proc.mctx)
			return nil
		}),
		next.Insert(r, func() error {
			a.merger.add(start(a.ctx, map[mutable.Context]async.Runner{proc.mctx: r}, []mutable.Context{proc.mctx})...)
			cancelFn()
			return nil
		}),
	)
	return ctx.Done()
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

func addListeners(listeners map[mutable.Context]chan mutable.Mutations, mc chan mutable.Mutations, ctxs ...mutable.Context) {
	for i := range ctxs {
		listeners[ctxs[i]] = mc
	}
}

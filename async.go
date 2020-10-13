package pipe

import (
	"context"

	"pipelined.dev/pipe/internal/async"
	"pipelined.dev/pipe/mutable"
)

type (
	// Async executes the pipe asynchronously. Every pipe component is
	// executed in its own goroutine.
	Async struct {
		ctx           context.Context
		cancelFn      context.CancelFunc
		mutability    mutable.Context
		bufferSize    int
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
		p.Lines[i].runners(ctx, p.bufferSize, mutationsChan, runners)
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
	runnerMutability := mutable.Mutable()
	go func() {
		defer close(errc)
		for {
			select {
			case ms := <-mutationsChan:
				for _, m := range ms {
					// mutate pipe itself
					if m.Context == runnerMutability {
						if err := m.Apply(); err != nil {
							cancelFn()
							merger.await()
							errc <- err
						}
					} else {
						for i := range ms {
							if c := listeners[m.Context]; c != nil {
								mutations[c] = mutations[c].Put(ms[i])
							}
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
		bufferSize:    p.bufferSize,
		mutability:    runnerMutability,
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
func (l *Line) runners(ctx context.Context, bufferSize int, mc chan mutable.Mutations, runners map[mutable.Context]async.Runner) {
	var r async.Runner
	r = async.Source(
		mc,
		l.Source.mutability,
		l.Source.Output.poolAllocator(bufferSize),
		async.SourceFunc(l.Source.SourceFunc),
		async.HookFunc(l.Source.StartFunc),
		async.HookFunc(l.Source.FlushFunc),
	)
	runners[l.Source.mutability] = r

	in := r.Out()
	props := l.Source.Output
	for i := range l.Processors {
		r = async.Processor(
			l.Processors[i].mutability,
			in,
			props.poolAllocator(bufferSize),
			l.Processors[i].Output.poolAllocator(bufferSize),
			async.ProcessFunc(l.Processors[i].ProcessFunc),
			async.HookFunc(l.Processors[i].StartFunc),
			async.HookFunc(l.Processors[i].FlushFunc),
		)
		runners[l.Processors[i].mutability] = r
		in = r.Out()
		props = l.Processors[i].Output
	}

	runners[l.Sink.mutability] = async.Sink(
		l.Sink.mutability,
		in,
		props.poolAllocator(bufferSize),
		async.SinkFunc(l.Sink.SinkFunc),
		async.HookFunc(l.Sink.StartFunc),
		async.HookFunc(l.Sink.FlushFunc),
	)
}

func (l *Line) mutableContexts() []mutable.Context {
	ctxs := make([]mutable.Context, 0, 2+len(l.Processors))

	ctxs = append(ctxs, l.Source.mutability, l.Sink.mutability)
	for i := range l.Processors {
		ctxs = append(ctxs, l.Processors[i].mutability)
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

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (a *Async) Push(mutations ...mutable.Mutation) {
	a.mutationsChan <- mutations
}

// AddLine adds the line to the pipe.
func (a *Async) AddLine(l *Line) mutable.Mutation {
	mc := make(chan mutable.Mutations, 1)
	l.runners(a.ctx, a.bufferSize, mc, a.runners)
	ctxs := l.mutableContexts()
	return a.mutability.Mutate(func() error {
		addListeners(a.listeners, mc, ctxs...)
		a.merger.add(start(a.ctx, a.runners, ctxs)...)
		return nil
	})
}

// AddProcessor adds the processor to the running line. Pos is the index
// where processor should be inserted relatively to other processors i.e:
// pos 0 means that new processor will be inserted right after the pipe
// source.
func (a *Async) AddProcessor(l *Line, pos int, fn ProcessorAllocatorFunc) error {
	// obtain signal properties of previous stage.
	var (
		output     SignalProperties
		prevRunner async.Runner
	)
	if pos == 0 {
		output = l.Source.Output
		prevRunner = a.runners[l.Source.mutability]
	} else {
		output = l.Processors[pos-1].Output
		prevRunner = a.runners[l.Processors[pos-1].mutability]
	}

	// allocate new processor
	m := mutable.Mutable()
	proc, err := fn(m, a.bufferSize, output)
	if err != nil {
		return err
	}
	proc.mutability = m

	// obtain mutable context of the next stage.
	var nextRunner async.Runner
	if pos == len(l.Processors) {
		nextRunner = a.runners[l.Sink.mutability]
	} else {
		nextRunner = a.runners[l.Processors[pos].mutability]
	}
	r := async.Processor(
		m,
		prevRunner.Out(),
		output.poolAllocator(a.bufferSize),
		proc.Output.poolAllocator(a.bufferSize),
		async.ProcessFunc(proc.ProcessFunc),
		async.HookFunc(proc.StartFunc),
		async.HookFunc(proc.FlushFunc),
	)
	// append processor
	l.Processors = append(l.Processors, Processor{})
	copy(l.Processors[pos+1:], l.Processors[pos:])
	l.Processors[pos] = proc
	a.Push(
		a.mutability.Mutate(func() error {
			addListeners(a.listeners, a.listeners[l.Source.mutability], m)
			return nil
		}),
		nextRunner.Insert(r, func() error {
			a.merger.add(start(a.ctx, map[mutable.Context]async.Runner{m: r}, []mutable.Context{m})...)
			return nil
		}),
	)
	return nil
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

package pipe

import (
	"context"

	"pipelined.dev/pipe/internal/async"
	"pipelined.dev/pipe/mutable"
)

type (
	// Async executes the pipe.
	Async struct {
		ctx           context.Context
		cancelFn      context.CancelFunc
		mutability    mutable.Context
		bufferSize    int
		merger        *merger
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
	runners := make([]async.Runner, 0, len(p.Lines)*3)
	listeners := make(map[mutable.Context]chan mutable.Mutations)
	for i := range p.Lines {
		mutationsChan := make(chan mutable.Mutations, 1)
		runners = append(runners,
			p.Lines[i].runners(ctx, p.bufferSize, mutationsChan)...,
		)
		addListeners(listeners, p.Lines[i], mutationsChan)
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
	merger := merger{
		errorChan: make(chan error, 1),
	}
	merger.merge(start(ctx, runners)...)
	go merger.wait()

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
	}
}

// Line binds components. All allocators are executed and wrapped into
// runners. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (l *Line) runners(ctx context.Context, bufferSize int, ms chan mutable.Mutations) []async.Runner {
	runners := make([]async.Runner, 0, 2+len(l.Processors))

	var r async.Runner
	r = async.Source(
		ms,
		l.Source.mutability,
		l.Source.Output.poolAllocator(bufferSize),
		async.SourceFunc(l.Source.SourceFunc),
		async.HookFunc(l.Source.StartFunc),
		async.HookFunc(l.Source.FlushFunc),
	)
	runners = append(runners, r)

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
		runners = append(runners, r)
		in = r.Out()
		props = l.Processors[i].Output
	}

	runners = append(runners,
		async.Sink(
			l.Sink.mutability,
			in,
			props.poolAllocator(bufferSize),
			async.SinkFunc(l.Sink.SinkFunc),
			async.HookFunc(l.Sink.StartFunc),
			async.HookFunc(l.Sink.FlushFunc),
		),
	)

	return runners
}

func push(mutations map[chan mutable.Mutations]mutable.Mutations) {
	for c, m := range mutations {
		c <- m
		delete(mutations, c)
	}
}

// start starts the execution of pipe.
func start(ctx context.Context, runners []async.Runner) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, len(runners))
	for i := range runners {
		errChans = append(errChans, runners[i].Run(ctx))
	}
	return errChans
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (r *Async) Push(mutations ...mutable.Mutation) {
	r.mutationsChan <- mutations
}

// AddLine adds the line to the pipe.
func (r *Async) AddLine(l *Line) mutable.Mutation {
	ms := make(chan mutable.Mutations, 1)
	rs := l.runners(r.ctx, r.bufferSize, ms)
	// r.runners[l] = lineRunner
	return r.mutability.Mutate(func() error {
		addListeners(r.listeners, l, ms) // RACE CONDITION
		r.merger.merge(start(r.ctx, rs)...)
		return nil
	})
}

func (r *Async) AddProcessor(l *Line, pos int, fn ProcessorAllocatorFunc) (mutable.Mutation, error) {
	panic("not implemented")
	// lineRunner, ok := r.runners[l]
	// if !ok {
	// 	panic("line is not running")
	// }

	// allocate processor with output properties of previous stage.
	var (
		input SignalProperties
		// mut   mutable.Mutability
	)
	if pos == 0 {
		input = l.Source.Output
		// mut = l.Source.mutability
	} else {
		input = l.Processors[pos-1].Output
		// mut = l.Processors[pos-1].mutability
	}
	m := mutable.Mutable()
	proc, err := fn(m, r.bufferSize, input)
	if err != nil {
		return mutable.Mutation{}, err
	}
	proc.mutability = m

	l.Processors = append(l.Processors, Processor{})
	copy(l.Processors[pos+1:], l.Processors[pos:])
	l.Processors[pos] = proc

	// procRunner, _ := proc.runner(r.bufferSize, input)
	// lineasync.AddProcessor(pos, procRunner)
	// r.listeners[proc.mutability] = lineasync.Mutations
	return proc.mutability.Mutate(func() error {
		// TODO: add magic here
		// r.merger.merge(procasync.Run(r.ctx))
		return nil
	}), nil
}

// Await for successful finish or first error to occur.
func (r *Async) Await() error {
	for err := range r.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

func addListeners(listeners map[mutable.Context]chan mutable.Mutations, l *Line, ms chan mutable.Mutations) {
	listeners[l.Source.mutability] = ms
	for i := range l.Processors {
		listeners[l.Processors[i].mutability] = ms
	}
	listeners[l.Sink.mutability] = ms
}

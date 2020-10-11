package pipe

import (
	"context"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/mutability"
)

type (
	// Runner executes the pipe.
	Runner struct {
		ctx           context.Context
		cancelFn      context.CancelFunc
		mutability    mutability.Mutability
		bufferSize    int
		merger        *merger
		listeners     map[mutability.Mutability]chan mutability.Mutations
		runners       map[mutability.Mutability]runner.Runner
		mutations     map[chan mutability.Mutations]mutability.Mutations
		mutationsChan chan []mutability.Mutation
		errorChan     chan error
	}
)

// Run creates and starts new pipe.
func (p *Pipe) Run(ctx context.Context, initializers ...mutability.Mutation) *Runner {
	ctx, cancelFn := context.WithCancel(ctx)
	runners := make([]runner.Runner, 0, len(p.Lines)*3)
	listeners := make(map[mutability.Mutability]chan mutability.Mutations)
	for i := range p.Lines {
		mutationsChan := make(chan mutability.Mutations, 1)
		runners = append(runners,
			p.Lines[i].runners(ctx, p.bufferSize, mutationsChan)...,
		)
		addListeners(listeners, p.Lines[i], mutationsChan)
	}
	mutations := make(map[chan mutability.Mutations]mutability.Mutations)
	for i := range initializers {
		if c := listeners[initializers[i].Mutability]; c != nil {
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
	mutationsChan := make(chan []mutability.Mutation, 1)
	runnerMutability := mutability.Mutable()
	go func() {
		defer close(errc)
		for {
			select {
			case ms := <-mutationsChan:
				for _, m := range ms {
					// mutate pipe itself
					if m.Mutability == runnerMutability {
						if err := m.Apply(); err != nil {
							cancelFn()
							merger.await()
							errc <- err
						}
					} else {
						for i := range ms {
							if c := listeners[m.Mutability]; c != nil {
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
	return &Runner{
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
func (l *Line) runners(ctx context.Context, bufferSize int, ms chan mutability.Mutations) []runner.Runner {
	runners := make([]runner.Runner, 0, 2+len(l.Processors))

	var r runner.Runner
	r = runner.Source(
		ms,
		l.Source.mutability,
		l.Source.Output.poolAllocator(bufferSize),
		runner.SourceFunc(l.Source.SourceFunc),
		runner.HookFunc(l.Source.StartFunc),
		runner.HookFunc(l.Source.FlushFunc),
	)
	runners = append(runners, r)

	in := r.Out()
	props := l.Source.Output
	for i := range l.Processors {
		r = runner.Processor(
			l.Processors[i].mutability,
			in,
			props.poolAllocator(bufferSize),
			l.Processors[i].Output.poolAllocator(bufferSize),
			runner.ProcessFunc(l.Processors[i].ProcessFunc),
			runner.HookFunc(l.Processors[i].StartFunc),
			runner.HookFunc(l.Processors[i].FlushFunc),
		)
		runners = append(runners, r)
		in = r.Out()
		props = l.Processors[i].Output
	}

	runners = append(runners,
		runner.Sink(
			l.Sink.mutability,
			in,
			props.poolAllocator(bufferSize),
			runner.SinkFunc(l.Sink.SinkFunc),
			runner.HookFunc(l.Sink.StartFunc),
			runner.HookFunc(l.Sink.FlushFunc),
		),
	)

	return runners
}

func push(mutations map[chan mutability.Mutations]mutability.Mutations) {
	for c, m := range mutations {
		c <- m
		delete(mutations, c)
	}
}

// start starts the execution of pipe.
func start(ctx context.Context, runners []runner.Runner) []<-chan error {
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
func (r *Runner) Push(mutations ...mutability.Mutation) {
	r.mutationsChan <- mutations
}

// AddLine adds the line to the pipe.
func (r *Runner) AddLine(l *Line) mutability.Mutation {
	ms := make(chan mutability.Mutations, 1)
	rs := l.runners(r.ctx, r.bufferSize, ms)
	// r.runners[l] = lineRunner
	return r.mutability.Mutate(func() error {
		addListeners(r.listeners, l, ms) // RACE CONDITION
		r.merger.merge(start(r.ctx, rs)...)
		return nil
	})
}

func (r *Runner) AddProcessor(l *Line, pos int, fn ProcessorAllocatorFunc) (mutability.Mutation, error) {
	panic("not implemented")
	// lineRunner, ok := r.runners[l]
	// if !ok {
	// 	panic("line is not running")
	// }

	// allocate processor with output properties of previous stage.
	var (
		input SignalProperties
		// mut   mutability.Mutability
	)
	if pos == 0 {
		input = l.Source.Output
		// mut = l.Source.mutability
	} else {
		input = l.Processors[pos-1].Output
		// mut = l.Processors[pos-1].mutability
	}
	m := mutability.Mutable()
	proc, err := fn(m, r.bufferSize, input)
	if err != nil {
		return mutability.Mutation{}, err
	}
	proc.mutability = m

	l.Processors = append(l.Processors, Processor{})
	copy(l.Processors[pos+1:], l.Processors[pos:])
	l.Processors[pos] = proc

	// procRunner, _ := proc.runner(r.bufferSize, input)
	// lineRunner.AddProcessor(pos, procRunner)
	// r.listeners[proc.mutability] = lineRunner.Mutations
	return proc.mutability.Mutate(func() error {
		// TODO: add magic here
		// r.merger.merge(procRunner.Run(r.ctx))
		return nil
	}), nil
}

// Wait for state transition or first error to occur.
func (r *Runner) Wait() error {
	for err := range r.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

func addListeners(listeners map[mutability.Mutability]chan mutability.Mutations, l *Line, ms chan mutability.Mutations) {
	listeners[l.Source.mutability] = ms
	for i := range l.Processors {
		listeners[l.Processors[i].mutability] = ms
	}
	listeners[l.Sink.mutability] = ms
}

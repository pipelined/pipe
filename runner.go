package pipe

import (
	"context"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/signal"
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
		mutations     map[chan mutability.Mutations]mutability.Mutations
		mutationsChan chan []mutability.Mutation
		errorChan     chan error
		runners       []runner.Runner
	}
)

// Run creates and starts new pipe.
func (p *Pipe) Run(ctx context.Context, initializers ...mutability.Mutation) *Runner {
	ctx, cancelFn := context.WithCancel(ctx)
	runners := make([]runner.Runner, 0, len(p.lines))
	listeners := make(map[mutability.Mutability]chan mutability.Mutations)
	for i := range p.lines {
		r := p.lines[i].runner(ctx, p.bufferSize)
		runners = append(runners, r)
		addListeners(listeners, r)
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
		bufferSize:    p.bufferSize,
		ctx:           ctx,
		cancelFn:      cancelFn,
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
func (l Line) runner(ctx context.Context, bufferSize int) runner.Runner {
	source, input := l.Source.runner(bufferSize)

	processors := make([]runner.Processor, 0, len(l.Processors))
	var processor runner.Processor
	for i := range l.Processors {
		processor, input = l.Processors[i].runner(bufferSize, input)
		processors = append(processors, processor)
	}

	sink := l.Sink.runner(bufferSize, input)
	return runner.Runner{
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}
}

func (s Source) runner(bufferSize int) (runner.Source, SignalProperties) {
	return runner.Source{
		Mutations:  make(chan mutability.Mutations, 1),
		Mutability: s.Mutability,
		OutPool:    signal.GetPoolAllocator(s.Output.Channels, bufferSize, bufferSize),
		Fn:         s.SourceFunc,
		Start:      runner.HookFunc(s.StartFunc),
		Flush:      runner.HookFunc(s.FlushFunc),
	}, s.Output
}

func (p Processor) runner(bufferSize int, input SignalProperties) (runner.Processor, SignalProperties) {
	return runner.Processor{
		Mutability: p.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		OutPool:    signal.GetPoolAllocator(p.Output.Channels, bufferSize, bufferSize),
		Fn:         p.ProcessFunc,
		Flush:      runner.HookFunc(p.FlushFunc),
	}, p.Output
}

func (s Sink) runner(bufferSize int, input SignalProperties) runner.Sink {
	return runner.Sink{
		Mutability: s.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		Fn:         s.SinkFunc,
		Flush:      runner.HookFunc(s.FlushFunc),
	}
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
	errChans := make([]<-chan error, 0, 2*len(runners))
	for i := range runners {
		errChans = append(errChans, runners[i].Run(ctx)...)
	}
	return errChans
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (r *Runner) Push(mutations ...mutability.Mutation) {
	r.mutationsChan <- mutations
}

// AddLine adds the line to the pipe.
func (r *Runner) AddLine(l Line) mutability.Mutation {
	return r.mutability.Mutate(func() error {
		runner := l.runner(r.ctx, r.bufferSize)
		addListeners(r.listeners, runner)
		r.merger.merge(runner.Run(r.ctx)...)
		return nil
	})
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

func addListeners(listeners map[mutability.Mutability]chan mutability.Mutations, r runner.Runner) {
	listeners[r.Source.Mutability] = r.Mutations
	for i := range r.Processors {
		listeners[r.Processors[i].Mutability] = r.Mutations
	}
	listeners[r.Sink.Mutability] = r.Mutations
}

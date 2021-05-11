package pipe

import (
	"context"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/mutable"
)

type (
	// Pipe is a graph formed with multiple lines of bound DSP components.
	Pipe struct {
		ctx        context.Context
		mctx       mutable.Context
		bufferSize int
		// async lines have runner per component
		// sync lines always wrapped in multiLineExecutor
		mutationsChan chan []mutable.Mutation
		pusher        mutable.Pusher
		executors     map[mutable.Context]executor
		errorMerger
		routes []*route
	}

	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		dest mutable.Destination
		mutable.Context
		SourceFunc
		StartFunc
		FlushFunc
		SignalProperties
		out
	}

	// SourceFunc takes the output buffer and fills it with a signal data.
	// If no data is available, io.EOF should be returned.
	SourceFunc func(out signal.Floating) (int, error)

	// Processor is a mutator of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Processor struct {
		mutable.Context
		ProcessFunc
		StartFunc
		FlushFunc
		SignalProperties
		in
		out
	}

	// ProcessFunc takes the input buffer, applies processing logic and
	// writes the result into output buffer.
	ProcessFunc func(in, out signal.Floating) (int, error)

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mutable.Context
		SinkFunc
		StartFunc
		FlushFunc
		SignalProperties
		in
	}

	// SinkFunc takes the input buffer and writes that to the underlying
	// destination.
	SinkFunc func(in signal.Floating) error

	// StartFunc provides a hook to flush all buffers for the component.
	StartFunc func(ctx context.Context) error
	// FlushFunc provides a hook to flush all buffers for the component or
	// execute any other form of finalization logic.
	FlushFunc func(ctx context.Context) error

	out struct {
		fitting.Sender
		*signal.PoolAllocator
	}

	in struct {
		fitting.Receiver
		*signal.PoolAllocator
	}
)

// New returns a new Pipe that binds multiple lines using the provided
// buffer size.
func New(bufferSize int, lines ...Line) (*Pipe, error) {
	if len(lines) == 0 {
		panic("pipe without lines")
	}
	routes := make([]*route, 0, len(lines))
	for _, l := range lines {
		r, err := l.route(bufferSize)
		if err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}

	return &Pipe{
		mctx:          mutable.Mutable(),
		mutationsChan: make(chan []mutable.Mutation, 1),
		bufferSize:    bufferSize,
		routes:        routes,
		pusher:        mutable.NewPusher(),
	}, nil
}

// Run executes the pipe in a single goroutine, sequentially.
func Run(ctx context.Context, bufferSize int, lines ...Line) error {
	e := multiLineExecutor{}
	mctx := mutable.Mutable()
	for _, l := range lines {
		l.Context = mctx
		r, err := l.route(bufferSize)
		if err != nil {
			return err
		}
		e.Lines = append(e.Lines, r.executor(nil))
	}
	return run(ctx, &e)
}

// Start starts the pipe execution.
func (p *Pipe) Start(ctx context.Context, initializers ...mutable.Mutation) <-chan error {
	// cancel is required to stop the pipe in case of error
	ctx, cancelFn := context.WithCancel(ctx)
	p.ctx = ctx
	p.errorMerger.errorChan = make(chan error, 1)
	p.executors = make(map[mutable.Context]executor)
	for _, r := range p.routes {
		r.bindExecutors(p.executors, p.pusher)
	}
	for _, e := range p.executors {
		e.connectOutputs()
	}
	// push initializers before start
	p.pusher.Put(initializers...)
	p.pusher.Push(ctx)
	for _, e := range p.executors {
		p.errorMerger.add(start(ctx, e))
	}
	go p.errorMerger.wait()
	errc := make(chan error, 1)
	go p.start(ctx, errc, cancelFn)
	return errc
}

func (p *Pipe) start(ctx context.Context, errc chan error, cancelFn context.CancelFunc) {
	defer close(errc)
	for {
		select {
		case ms := <-p.mutationsChan:
			for _, m := range ms {
				// mutate the pipe itself
				if m.Context == p.mctx {
					m.Apply()
				} else {
					p.pusher.Put(m)
				}
			}
			p.pusher.Push(ctx)
		case err, ok := <-p.errorMerger.errorChan:
			// merger has buffer of one error, if more errors happen, they
			// will be ignored.
			if ok {
				cancelFn()
				p.errorMerger.drain()
				errc <- err
			}
			return
		}
	}
}

// Push new mutators into pipe. Calling this method after pipe is done will
// cause a panic.
func (p *Pipe) Push(mutations ...mutable.Mutation) {
	p.mutationsChan <- mutations
}

// Wait for successful finish or first error to occur.
func Wait(errc <-chan error) error {
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

// AddLine creates the line for provied route and adds it to the pipe.
func (p *Pipe) AddLine(l Line) mutable.Mutation {
	return p.mctx.Mutate(func() error {
		r, err := l.route(p.bufferSize)
		if err != nil {
			return err
		}

		if !l.Context.IsMutable() {
			r.bindExecutors(p.executors, p.pusher)
			// connect all fittings
			r.source.connectOutputs()
			for _, proc := range r.processors {
				proc.connectOutputs()
			}
			r.sink.connectOutputs()

			// start all executors
			p.errorMerger.add(start(p.ctx, &r.source))
			for _, proc := range r.processors {
				p.errorMerger.add(start(p.ctx, &proc))
			}
			p.errorMerger.add(start(p.ctx, &r.sink))
			return nil
		}
		// add to existing goroutine
		if e, ok := p.executors[l.Context]; ok {
			mle := e.(*multiLineExecutor)
			p.pusher.Put(
				mle.Context.Mutate(
					func() error {
						mle.Lines = append(mle.Lines, r.executor(mle.Destination))
						return nil
					},
				),
			)
			return nil
		}
		// new goroutine
		d := mutable.NewDestination()
		p.pusher.AddDestination(r.context, d)
		e := &multiLineExecutor{
			Context:     r.context,
			Destination: d,
			Lines:       []*lineExecutor{r.executor(d)},
		}
		p.executors[r.context] = e
		p.errorMerger.add(start(p.ctx, e))
		return nil
	})
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
	return processors
}

func (s *Source) connectOutputs() {
	s.out.Sender.Bind()
}

// Execute does a single iteration of source component. io.EOF is returned
// if context is done.
func (s *Source) execute(ctx context.Context) error {
	var ms mutable.Mutations
	select {
	case ms = <-s.dest:
		if err := ms.ApplyTo(s.Context); err != nil {
			return err
		}
	case <-ctx.Done():
		s.out.Close()
		return io.EOF
	default:
	}

	output := s.out.Float64()
	var (
		read int
		err  error
	)
	if read, err = s.SourceFunc(output); err != nil {
		s.out.Close()
		output.Free(s.out.PoolAllocator)
		return err
	}
	if read != output.Length() {
		output = output.Slice(0, read)
	}

	if !s.out.Send(ctx, fitting.Message{Signal: output, Mutations: ms}) {
		s.out.Close()
		return io.EOF
	}
	return nil
}

func (p *Processor) connectOutputs() {
	p.out.Sender.Bind()
}

// Execute does a single iteration of processor component. io.EOF is
// returned if context is done.
func (p *Processor) execute(ctx context.Context) error {
	m, ok := p.in.Receive(ctx)
	if !ok {
		p.out.Close()
		return io.EOF
	}
	defer m.Signal.Free(p.in.PoolAllocator)

	if err := m.Mutations.ApplyTo(p.Context); err != nil {
		return err
	}

	output := p.out.Float64()
	if processed, err := p.ProcessFunc(m.Signal, output); err != nil {
		p.out.Close()
		return err
	} else if processed != p.out.Length {
		output = output.Slice(0, processed)
	}

	if !p.out.Send(ctx, fitting.Message{Signal: output, Mutations: m.Mutations}) {
		p.out.Close()
		output.Free(p.out.PoolAllocator)
		return io.EOF
	}
	return nil
}

func (s *Sink) connectOutputs() {}

// Execute does a single iteration of sink component. io.EOF is returned if
// context is done.
func (s *Sink) execute(ctx context.Context) error {
	m, ok := s.in.Receive(ctx)
	if !ok {
		return io.EOF
	}
	defer m.Signal.Free(s.in.PoolAllocator)
	if err := m.Mutations.ApplyTo(s.Context); err != nil {
		return err
	}

	err := s.SinkFunc(m.Signal)
	return err
}

// startHook calls the start hook.
func (fn StartFunc) startHook(ctx context.Context) error {
	return callHook(ctx, fn)
}

// flushHook calls the flush hook.
func (fn FlushFunc) flushHook(ctx context.Context) error {
	return callHook(ctx, fn)
}

func callHook(ctx context.Context, hook func(context.Context) error) error {
	if hook == nil {
		return nil
	}
	return hook(ctx)
}

func (sp SignalProperties) poolAllocator(bufferSize int) *signal.PoolAllocator {
	return signal.GetPoolAllocator(sp.Channels, bufferSize, bufferSize)
}

package pipe

import (
	"context"
	"fmt"
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
	for i, l := range lines {
		l.Context = mctx
		r, err := l.route(bufferSize)
		if err != nil {
			return err
		}
		r.connect(bufferSize)
		e.executors = append(e.executors, r.executor(nil, i))
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
	for i, r := range p.routes {
		p.addExecutors(r, i)
		r.connect(p.bufferSize)
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
func (p *Pipe) AddLine(l Line) <-chan struct{} {
	if p.ctx == nil {
		panic("pipe isn't running")
	}
	ctx, cancelFn := context.WithCancel(p.ctx)
	p.Push(p.mctx.Mutate(func() error {
		r, err := l.route(p.bufferSize)
		if err != nil {
			return fmt.Errorf("error adding line: %w", err)
		}
		routeIdx := len(p.routes)
		p.routes = append(p.routes, r)
		// connect all fittings
		r.connect(p.bufferSize)

		// async line
		if !l.Context.IsMutable() {
			p.addExecutors(r, routeIdx)
			// start all executors
			p.errorMerger.add(start(p.ctx, r.source))
			for _, proc := range r.processors {
				p.errorMerger.add(start(p.ctx, proc))
			}
			p.errorMerger.add(start(p.ctx, r.sink))
			cancelFn()
			return nil
		}
		// sync add to existing goroutine
		if e, ok := p.executors[l.Context]; ok {
			p.pusher.Put(e.(*multiLineExecutor).addRoute(p.ctx, r, routeIdx, cancelFn))
			return nil
		}
		// sync new goroutine
		e := p.newMultilineExecutor(r, routeIdx)
		p.executors[r.context] = e
		p.errorMerger.add(start(p.ctx, e))
		cancelFn()
		return nil
	}))
	return ctx.Done()
}

func (p *Pipe) addExecutors(r *route, routeIdx int) {
	if r.context.IsMutable() {
		if e, ok := p.executors[r.context]; ok {
			mle := e.(*multiLineExecutor)
			mle.executors = append(mle.executors, r.executor(mle.Destination, routeIdx))
			return
		}
		p.newMultilineExecutor(r, routeIdx)
		return
	}

	d := mutable.NewDestination()
	r.source.dest = d
	p.pusher.AddDestination(r.source.Context, d)
	p.executors[r.source.Context] = r.source
	for i := range r.processors {
		p.pusher.AddDestination(r.processors[i].Context, d)
		p.executors[r.processors[i].Context] = r.processors[i]
	}
	p.pusher.AddDestination(r.sink.Context, d)
	p.executors[r.sink.Context] = r.sink
}

func (p *Pipe) newMultilineExecutor(r *route, routeIdx int) *multiLineExecutor {
	d := mutable.NewDestination()
	p.pusher.AddDestination(r.context, d)
	e := multiLineExecutor{
		Context:     r.context,
		Destination: d,
		executors:   []*lineExecutor{r.executor(d, routeIdx)},
	}
	p.executors[r.context] = &e
	return &e
}

func (p *Pipe) InsertProcessor(line, pos int, procAlloc ProcessorAllocatorFunc) <-chan struct{} {
	if p.ctx == nil {
		panic("pipe isn't running")
	}
	ctx, cancelFn := context.WithCancel(p.ctx)
	p.Push(p.mctx.Mutate(func() error {
		r := p.routes[line]
		// allocate and connect
		prevProps, prevOut := r.prev(pos)
		mctx := componentContext(r.context)
		proc, err := procAlloc.allocate(mctx, p.bufferSize, prevProps)
		if err != nil {
			cancelFn()
			return err
		}

		if r.context.IsMutable() {
			proc.connect(p.bufferSize, fitting.Sync, prevOut)
			mle := p.executors[r.context].(*multiLineExecutor)
			p.pusher.Put(mle.Context.Mutate(func() error {
				defer cancelFn()
				pos++ // slice of executors includes source and sink
				for _, e := range mle.executors {
					if e.route != line {
						continue
					}

					inserter := e.executors[pos].(interface{ insert(out) })
					inserter.insert(proc.out)

					err := proc.startHook(p.ctx)
					if err != nil {
						return fmt.Errorf("error starting processor: %w", err)
					}
					e.executors = append(e.executors, nil)
					copy(e.executors[pos+1:], e.executors[pos:])
					e.executors[pos] = &proc
					e.started++
					break
				}
				return nil
			}))
			return nil
		}

		proc.connect(p.bufferSize, fitting.Async, prevOut)

		// get ready for start
		p.executors[mctx] = &proc
		p.pusher.AddDestination(mctx, r.source.dest)

		// add to route after this function returns
		defer func() {
			r.processors = append(r.processors, nil)
			copy(r.processors[pos+1:], r.processors[pos:])
			r.processors[pos] = &proc
		}()
		if pos == len(r.processors) {
			p.pusher.Put(r.sink.Mutate((func() error {
				r.sink.in.insert(proc.out)
				p.errorMerger.add(start(p.ctx, &proc))
				cancelFn()
				return nil
			})))
			return nil
		}
		nextProc := r.processors[pos]
		p.pusher.Put(nextProc.Mutate((func() error {
			nextProc.in.insert(proc.out)
			p.errorMerger.add(start(p.ctx, &proc))
			cancelFn()
			return nil
		})))

		return nil
	}))
	return ctx.Done()
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
	return processors
}

func (s *Source) connect(bufferSize int, fn fitting.New) {
	s.out = out{
		allocator: s.SignalProperties.poolAllocator(bufferSize),
		sender:    fn(),
	}
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
		s.out.sender.Close()
		return io.EOF
	default:
	}

	output := s.out.allocator.Float64()
	var (
		read int
		err  error
	)
	if read, err = s.SourceFunc(output); err != nil {
		s.out.sender.Close()
		output.Free(s.out.allocator)
		return err
	}
	if read != output.Length() {
		output = output.Slice(0, read)
	}

	if !s.out.sender.Send(ctx, fitting.Message{Signal: output, Mutations: ms}) {
		s.out.sender.Close()
		return io.EOF
	}
	return nil
}

func (p *Processor) connect(bufferSize int, fn fitting.New, prevOut out) {
	p.in.insert(prevOut)
	p.out = out{
		allocator: p.SignalProperties.poolAllocator(bufferSize),
		sender:    fn(),
	}
}

// Execute does a single iteration of processor component. io.EOF is
// returned if context is done.
func (p *Processor) execute(ctx context.Context) error {
	m, ok := p.in.receiver.Receive(ctx)
	if !ok {
		p.out.sender.Close()
		return io.EOF
	}
	defer m.Signal.Free(p.in.allocator)

	if err := m.Mutations.ApplyTo(p.Context); err != nil {
		return err
	}

	output := p.out.allocator.Float64()
	if processed, err := p.ProcessFunc(m.Signal, output); err != nil {
		p.out.sender.Close()
		return err
	} else if processed != p.out.allocator.Length {
		output = output.Slice(0, processed)
	}

	if !p.out.sender.Send(ctx, fitting.Message{Signal: output, Mutations: m.Mutations}) {
		p.out.sender.Close()
		output.Free(p.out.allocator)
		return io.EOF
	}
	return nil
}

func (s *Sink) connect(bufferSize int, prevOut out) {
	s.in.insert(prevOut)
}

// Execute does a single iteration of sink component. io.EOF is returned if
// context is done.
func (s *Sink) execute(ctx context.Context) error {
	m, ok := s.in.receiver.Receive(ctx)
	if !ok {
		return io.EOF
	}
	defer m.Signal.Free(s.in.allocator)
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

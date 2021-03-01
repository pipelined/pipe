/*
Package pipe allows to build and execute DSP pipelines.

Concept

This package offers an opinionated perspective to DSP. It's based on the
idea that the signal processing can have up to three stages:

    Source - the origin of signal;
    Processor - the manipulator of the signal;
    Sink - the destination of signal;

It implies the following constraints:

    Source and Sink are mandatory;
    There might be 0 to n Processors;
    All stages are executed sequentially.

Current implementation supports two execution modes: sync and async. In
async mode every stage of every line is executed in by its own goroutine
and channels are used to communicate between them. Sync mode allows to run
one or more lines in the same goroutine. In this case lines and stages
within lines are executed sequentially, as-provided. Pipe allows to use
different modes in the same run.

Components

Each stage in the pipeline is implemented by components. For example,
wav.Source reads signal from wav file and vst2.Processor processes signal
with vst2 plugin. Components are instantiated with allocator functions:

    SourceAllocatorFunc
    ProcessorAllocatorFunc
    SinkAllocatorFunc

Allocator functions return component structures and pre-allocate all
required resources and structures. It reduces number of allocations during
pipeline execution and improves latency.

Component structures consist of mutability, run closure and flush hook. Run
closure is the function which will be called during the pipeline run. Flush
hook is triggered when pipe is done or interruped by error or timeout. It
enables to execute proper clean up logic. For mutability, refer to
mutability package documentation.

Line definition and pipe

To run the pipeline, one first need to build it. It starts with a line
definition:

    l1 := pipe.Line{
        Source: wav.Source(reader),
        Processors: pipe.Processors(
            vst.Processor(vst2.Host{}).Allocator(nil),
        ),
        Sink: wav.Sink(writer),
    }

Line defines the order in which DSP components form the pipeline. Once line
is defined, components can be bound together. It's done by creating a pipe:

    p, err := pipe.New(bufferSize, l1)

New executes all allocators provided by lines and binds components together
into the pipe.

Execution

Once pipe is built, it can be executed. To do that Start method should be
called:

    errc := p.Start()
    err := pipe.Wait(errc)

Start will start and asynchronously run all DSP components until either any
of the following things happen: the source is done; the context is done; an
error in any of the components occured.
*/
package pipe

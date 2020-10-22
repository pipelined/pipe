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

Current implementation supports only asynchronous execution, when every
stage is running in its own goroutine. It is inspired with the pipeline
pattern explained in the go blog https://blog.golang.org/pipelines.

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

Routing and binding

To run the pipeline, one first need to build it. It starts with a routing:

    r1 := pipe.Routing{
        Source: wav.Source(reader),
        Processors: pipe.Processors(
            vst2.Open(vstPath1),
            vst2.Open(vstPath2),
        ),
        Sink: wav.Sink(writer),
    }

Routing defines the order in which DSP components form the pipeline. Once
routing is defined, components can be bound together. It's done by creating
a pipe:

    p, err := pipe.New(bufferSize, r1)

New executes all allocators provided by routings and binds components
together into the pipe.

Execution

Once pipe is built, it can be executed. To do that Async method should be
called:

    r := p.Async()
    err := r.Await()

Async will start and asynchronously run all DSP components until either any
of the following things happen: the source is done; the context is done; an
error in any of the components occured.
*/
package pipe

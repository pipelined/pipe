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

The implementation is based on the pipeline pattern explained in the go
blog https://blog.golang.org/pipelines. Which means that stages are also
executed asynchronously.

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

To run the pipeline, one first need to build it. It starts with a line:

    line := pipe.Line{
        Source: wav.Source(reader),
        Processors: pipe.Processors(
            vst2.Open(vstPath1),
            vst2.Open(vstPath2),
        ),
        Sink: wav.Sink(writer),
    }

Line defines the order in which DSP components form the pipeline. Once line
is defined, components can be bound together. It's done by creating a pipe:

    p, err := pipe.New(context.Background(), bufferSize, line)

New executes all allocators provided in routing and binds components
together.

Execution

Once pipelined is built, it can be executed. To do that Run method should
be called:

    r := p.Run()
    err := r.Wait()

Run will start and asynchronously run all DSP components until either any
of the following things happen: source is done; context is done; error
occured.
*/
package pipe

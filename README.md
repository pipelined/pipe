![](pipe.png)

[![PkgGoDev](https://pkg.go.dev/badge/pipelined.dev/pipe)](https://pkg.go.dev/pipelined.dev/pipe)
[![Test](https://github.com/pipelined/pipe/workflows/Test/badge.svg)](https://github.com/pipelined/pipe/actions?query=workflow%3ATest)
[![Go Report Card](https://goreportcard.com/badge/pipelined.dev/pipe)](https://goreportcard.com/report/pipelined.dev/pipe)
[![codecov](https://codecov.io/gh/pipelined/pipe/branch/master/graph/badge.svg)](https://codecov.io/gh/pipelined/pipe)

`pipe` is a framework for floating point signal processing. It utilizes [pipeline](https://blog.golang.org/pipelines) pattern to build fast, asynchronomous and easy-to-extend pipes for sound processing. Each pipe consists of one Source, zero or multiple Processors and one or more Sinks. Phono also offers a friendly API for new pipe components implementations.

![diagram](https://dudk.github.io/post/lets-go/pipe_diagram.png)

## Getting started

Find examples in [example](https://github.com/pipelined/example) repository.

## Contributing

For a complete guide to contributing to `pipe`, see the [Contribution guide](https://github.com/pipelined/pipe/blob/master/CONTRIBUTING.md)

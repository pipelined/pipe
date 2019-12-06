![](pipe.png)

[![GoDoc](https://godoc.org/pipelined.dev/pipe?status.svg)](https://godoc.org/pipelined.dev/pipe)
[![Build Status](https://travis-ci.org/pipelined/pipe.svg?branch=master)](https://travis-ci.org/pipelined/pipe)
[![Go Report Card](https://goreportcard.com/badge/pipelined.dev/pipe)](https://goreportcard.com/report/pipelined.dev/pipe)
[![codecov](https://codecov.io/gh/pipelined/pipe/branch/master/graph/badge.svg)](https://codecov.io/gh/pipelined/pipe)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fpipelined%2Fphono.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fpipelined%2Fphono?ref=badge_shield)

`pipe` is a framework for floating point signal processing. It utilizes [pipeline](https://blog.golang.org/pipelines) pattern to build fast, asynchronomous and easy-to-extend pipes for sound processing. Each pipe consists of one Pump, zero or multiple Processors and one or more Sinks. Phono also offers a friendly API for new pipe components implementations.

![diagram](https://dudk.github.io/post/lets-go/pipe_diagram.png)

## Getting started

Find examples in [example](https://github.com/pipelined/example) repository.

## Testing

[mock](https://godoc.org/pipelined.dev/mock) package can be used to implement integration tests for custom Pumps, Processors and Sinks. It allows to mock up pipe components and then assert the data metrics.

## Contributing

For a complete guide to contributing to `pipe`, see the [Contribution guide](https://github.com/pipelined/pipe/blob/master/CONTRIBUTING.md)

## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fpipelined%2Fpipe.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fpipelined%2Fpipe?ref=badge_large)
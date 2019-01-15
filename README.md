![](phono.png)

[![GoDoc](https://godoc.org/github.com/pipelined/phono?status.svg)](https://godoc.org/github.com/pipelined/phono)
[![Build Status](https://travis-ci.org/pipelined/phono.svg?branch=master)](https://travis-ci.org/pipelined/phono)
[![Go Report Card](https://goreportcard.com/badge/github.com/pipelined/phono)](https://goreportcard.com/report/github.com/pipelined/phono)
[![codecov](https://codecov.io/gh/pipelined/phono/branch/master/graph/badge.svg)](https://codecov.io/gh/pipelined/phono)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fpipelined%2Fphono.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fpipelined%2Fphono?ref=badge_shield)

`phono` is a framework for floating point signal processing. It utilizes [pipeline](https://blog.golang.org/pipelines) pattern to build fast, asynchronomous and easy-to-extend pipes for sound processing. Each pipe consists of one Pump, zero or multiple Processors and one or more Sinks. Phono also offers a friendly API for new pipe components implementations.

![diagram](https://dudk.github.io/post/lets-go/pipe_diagram.png)

## Getting started

Find examples in [phono/_example](https://github.com/pipelined/phono/blob/master/_example) package:

* Read wav file and play it with portaudio [link](https://github.com/pipelined/phono/blob/master/_example/example1.go)
* Read wav file, process it with vst2 plugin and save to another wav file [link](https://github.com/pipelined/phono/blob/master/_example/example2.go)
* Read two wav files, mix them and save result into new wav file [link](https://github.com/pipelined/phono/blob/master/_example/example3.go)
* Read wav file, sample it, compose track, save result into wav file [link](https://github.com/pipelined/phono/blob/master/_example/example4.go)

## Implemented packages

Please, note that since `phono` is in active development stage, all packages are in experimental state. One of the main focus points is the stability of all listed and future packages.

1. `phono/wav` - Pump and Sink to read/write files
2. `phono/vst2` - Processor to utilize plugins
3. `phono/mixer` - simple mixer without balance and volume settings, Sink for inputs and Pump for output
4. `phono/asset` - structures to reuse Buffers
5. `phono/track` - Sink for sequential reads of asset and its slices
6. `phono/portaudio` - Sink for playback

## Dependencies

`phono/vst2` and `portaudio` packages do have external non-go dependencies. To use these packages, please check the documentation:

* [vst2](https://github.com/pipelined/vst2#dependencies)
* [portaudio](https://github.com/gordonklaus/portaudio#portaudio)

## Testing

[phono/mock](https://godoc.org/github.com/pipelined/phono/mock) package can be used to test custom Pumps, Processors and Sinks. It allows to mock up pipe elements and then assert the data metrics.

[phono/wav](https://godoc.org/github.com/pipelined/phono/wav) package contains Pump and Sink which allows to read/write wav files respectively. Its [test](https://github.com/pipelined/phono/blob/master/wav/wav_test.go) is a good example of [phono/mock](https://godoc.org/github.com/pipelined/phono/mock) package in use.

## Contributing

For a complete guide to contributing to `phono`, see the [Contribution giude](https://github.com/pipelined/phono/blob/master/CONTRIBUTING.md)


## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fpipelined%2Fphono.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fpipelined%2Fphono?ref=badge_large)
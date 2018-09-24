# `phono`
[![GoDoc](https://godoc.org/github.com/dudk/phono?status.svg)](https://godoc.org/github.com/dudk/phono)
[![Build Status](https://travis-ci.org/dudk/phono.svg?branch=master)](https://travis-ci.org/dudk/phono)
[![Go Report Card](https://goreportcard.com/badge/github.com/dudk/phono)](https://goreportcard.com/report/github.com/dudk/phono)

`phono` is a framework for floating point signal processing. It utilizes [pipeline](https://blog.golang.org/pipelines) pattern to build fast, asynchronomous and easy-to-extend pipes for sound processing. Each pipe consists of one Pump, zero or multiple Processors and one or more Sinks. Phono also offers a friendly API for new pipe components implementations.

![diagram](https://dudk.github.io/post/lets-go/pipe_diagram.png)

## Getting started

Find examples in [phono/example](https://github.com/dudk/phono/blob/master/example) package:

* Read wav file and play it with portaudio [link](https://github.com/dudk/phono/blob/master/example/example1.go)
* Read wav file, process it with vst2 plugin and save to another wav file [link](https://github.com/dudk/phono/blob/master/example/example2.go)
* Read two wav files, mix them and save result into new wav file [link](https://github.com/dudk/phono/blob/master/example/example3.go)
* Read wav file, sample it, compose track, save result into wav file [link](https://github.com/dudk/phono/blob/master/example/example4.go)

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

* [vst2](https://github.com/dudk/vst2#dependencies)
* [portaudio](https://github.com/gordonklaus/portaudio#portaudio)

## Testing

[phono/mock](https://godoc.org/github.com/dudk/phono/mock) package can be used to test custom Pumps, Processors and Sinks. It allows to mock up pipe elements and then assert the data metrics.

[phono/wav](https://godoc.org/github.com/dudk/phono/wav) package contains Pump and Sink which allows to read/write wav files respectively. Its [test](https://github.com/dudk/phono/blob/master/wav/wav_test.go) is a good example of [phono/mock](https://godoc.org/github.com/dudk/phono/mock) package in use.

## Contributing

For a complete guide to contributing to `phono`, see the [Contribution giude](https://github.com/dudk/phono/blob/master/CONTRIBUTING.md)
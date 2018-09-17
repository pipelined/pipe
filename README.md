# `phono`
[![Build Status](https://travis-ci.org/dudk/phono.svg?branch=master)](https://travis-ci.org/dudk/phono)
[![GoDoc](https://godoc.org/github.com/dudk/phono?status.svg)](https://godoc.org/github.com/dudk/phono)

`phono` is a framework for floating point signal processing. It utilizes [pipeline](https://blog.golang.org/pipelines) pattern to build fast, asynchronomous and easy-to-extend pipes to process sound. Each pipe consists of one Pump, zero or multiple Processors and one or more Sinks. Phono also offers a friendly API for new pipe components implementations.

## Example

Read wav file, process its content with vst2 plugin, save result into new wav file and also play it with audio device.

To accomplish this we can build a pipe which will have wav.Pump, vst2.Processor and wav.Sink with portaudio.Sink as of final stage:

_Note:_ to build vst2 package, please follow the [instruction](https://github.com/dudk/vst2) from ```dudk/vst2``` package

```go
package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/vst2"
	"github.com/dudk/phono/wav"
	vst2sdk "github.com/dudk/vst2"
)

// Example:
//		Read .wav file
//		Process it with VST2 plugin
// 		Save result into new .wav file
//		Play result with portaudio
func example() {
	// init wav Pump
	inPath := "_testdata/sample1.wav"
	bufferSize := phono.BufferSize(512)
	wavPump, err := wav.NewPump(
		inPath,
		bufferSize,
	)
	check(err)

	// open vst2 library
	vst2path := "_testdata/Krush.vst"
	vst2lib, err := vst2sdk.Open(vst2path)
	check(err)
	defer vst2lib.Close()

	// instantiate vst2 plugin
	vst2plugin, err := vst2lib.Open()
	check(err)
	defer vst2plugin.Close()

	// init vst2 Processor
	vst2processor := vst2.NewProcessor(
		vst2plugin,
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)

	// init wav Sink
	outPath := "_testdata/example2_out.wav"
	wavSink, err := wav.NewSink(
		outPath,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
		wavPump.WavBitDepth(),
		wavPump.WavAudioFormat(),
	)
	check(err)

	// init portaudio Sink
	paSink := portaudio.NewSink(bufferSize, wavPump.WavSampleRate(), wavPump.WavNumChannels())

	// build the pipe
	p := pipe.New(
		pipe.WithPump(wavPump),
		pipe.WithProcessors(vst2processor),
		pipe.WithSinks(paSink),
	)
	defer p.Close()

	// run the pipe
	err = p.Do(pipe.Run)
	check(err)
}
```

Find more examples in [phono/example](https://github.com/dudk/phono/example) package. 

## Implemented packages

Please, note that since `phono` is in active development stage, all packages are in experimental state. One of the main focus points is the stability of all listed and future packages.
1. `phono/wav` - Pump and Sink to read/write files
2. `phono/vst2` - Processor to utilize plugins
3. `phono/mixer` - simple mixer without balance and volume settings, Sink for inputs and Pump for output
4. `phono/asset` - structures to reuse Buffers
5. `phono/track` - Sink for sequential reads of asset and its slices
6. `phono/portaudio` - Sink for playback

### Testing
To test custom Pumps, Processors and Sinks - [`phono/mock`](https://github.com/dudk/phono/mock) package can be utilized. The purpose of this package is to provide useful pipe mocks to test pumps, processors and sinks.

#### Example
[phono/wav](https://github.com/dudk/phono/wav) package contains Pump and Sink which allows to read/write wav files respectively. To test both components next tests structure were defined:

```go
var tests = []struct {
	phono.BufferSize
	inFile   string
	outFile  string
	messages uint64
	samples  uint64
}{
	{
		BufferSize: 512,
		inFile:     "_testdata/sample1.wav",
		outFile:    "_testdata/out.wav",
		messages:   uint64(646),
		samples:    uint64(330534),
	},
	{
		BufferSize: 512,
		inFile:     "_testdata/out.wav",
		outFile:    "_testdata/out1.wav",
		messages:   uint64(646),
		samples:    uint64(330534),
	},
}
```
And test function uses mock.Processor to validate information passed through pipe:
```go
func TestWavPipe(t *testing.T) {
	for _, test := range tests {
		pump, err := wav.NewPump(test.inFile, bufferSize)
		assert.Nil(t, err)
		sink := wav.NewSink(test.outFile, pump.WavSampleRate(), pump.WavNumChannels(), pump.WavBitDepth(), pump.WavAudioFormat())

		processor := &mock.Processor{}
		p := pipe.New(
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		err = pipe.Do(p.Run)
		assert.Nil(t, err)
		err = p.Wait(pipe.Ready)
		assert.Nil(t, err)
		messageCount, sampleCount := processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, sampleCount)
	}
}
```

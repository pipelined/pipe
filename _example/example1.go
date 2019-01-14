package example

import (
	"github.com/pipelined/phono"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
)

// Example:
//		Read .wav file
//		Play it with portaudio
func one() {
	bufferSize := phono.BufferSize(512)
	// wav pump
	wavPump, err := wav.NewPump(
		test.Data.Wav1,
		bufferSize,
	)
	check(err)
	// take wav's sample rate as base
	sampleRate := wavPump.WavSampleRate()

	// portaudio sink
	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)

	// build pipe
	p := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump),
		pipe.WithSinks(paSink),
	)
	defer p.Close()

	// run pipe
	err = pipe.Wait(p.Run())
	check(err)
}

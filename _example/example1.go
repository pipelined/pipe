package example

import (
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
)

// Example:
//		Read .wav file
//		Play it with portaudio
func one() {
	bufferSize := 512
	// wav pump
	wavPump, err := wav.NewPump(
		test.Data.Wav1,
		bufferSize,
	)
	check(err)
	// take wav's sample rate as base
	sampleRate := wavPump.SampleRate()

	// portaudio sink
	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.SampleRate(),
		wavPump.NumChannels(),
	)

	// build pipe
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump),
		pipe.WithSinks(paSink),
	)
	check(err)
	defer p.Close()

	// run pipe
	err = pipe.Wait(p.Run())
	check(err)
}

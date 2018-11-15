package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/test"
	"github.com/dudk/phono/wav"
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

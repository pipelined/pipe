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
	wavPump := wav.NewPump(
		test.Data.Wav1,
	)

	// portaudio sink
	paSink := portaudio.NewSink()

	// build pipe
	p, err := pipe.New(
		bufferSize,
		pipe.WithPump(wavPump),
		pipe.WithSinks(paSink),
	)
	check(err)
	defer p.Close()

	// run pipe
	err = pipe.Wait(p.Run())
	check(err)
}

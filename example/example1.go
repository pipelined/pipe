package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/wav"
)

// Example 1:
//		Read .wav file
//		Play it with portaudio
func one() {
	wavPath := "../_testdata/sample1.wav"
	bufferSize := phono.BufferSize(512)
	wavPump, err := wav.NewPump(
		wavPath,
		bufferSize,
	)
	check(err)

	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)

	p := pipe.New(
		pipe.WithPump(wavPump),
		pipe.WithSinks(paSink),
	)
	defer p.Close()
	err = p.Do(pipe.Run)
	check(err)
}

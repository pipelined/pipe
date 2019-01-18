package example

import (
	"github.com/pipelined/phono/asset"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/track"
	"github.com/pipelined/phono/wav"
)

// Example:
//		Read .wav file
// 		Split it to samples
// 		Put samples to track
//		Save track into .wav and play it with portaudio
func four() {
	bufferSize := 512

	// wav pump
	wavPump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	check(err)
	sampleRate := wavPump.SampleRate()

	// asset sink
	asset := asset.New()

	// import pipe
	importAsset, err := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump),
		pipe.WithSinks(asset),
	)
	check(err)
	defer importAsset.Close()

	err = pipe.Wait(importAsset.Run())
	check(err)

	// track pump
	track := track.New(bufferSize, asset.NumChannels())

	// add samples
	track.AddClip(198450, asset.Clip(0, 44100))
	track.AddClip(66150, asset.Clip(44100, 44100))
	track.AddClip(132300, asset.Clip(0, 44100))

	// wav sink
	wavSink, err := wav.NewSink(
		test.Out.Example4,
		wavPump.SampleRate(),
		wavPump.NumChannels(),
		wavPump.BitDepth(),
		wavPump.Format(),
	)
	// portaudio sink
	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.SampleRate(),
		wavPump.NumChannels(),
	)

	// final pipe
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(track),
		pipe.WithSinks(wavSink, paSink),
	)
	check(err)

	err = pipe.Wait(p.Run())
	check(err)
}

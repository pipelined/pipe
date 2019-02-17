package example

import (
	"github.com/pipelined/phono/audio"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/portaudio"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
	"github.com/pipelined/signal"
)

// Example:
//		Read .wav file
// 		Split it to samples
// 		Put samples to track
//		Save track into .wav and play it with portaudio
func four() {
	bufferSize := 512

	// wav pump
	wavPump := wav.NewPump(test.Data.Wav1)

	// asset sink
	asset := &audio.Asset{}

	// import pipe
	importAsset, err := pipe.New(
		bufferSize,
		pipe.WithPump(wavPump),
		pipe.WithSinks(asset),
	)
	check(err)
	defer importAsset.Close()

	err = pipe.Wait(importAsset.Run())
	check(err)

	// track pump
	track := audio.NewTrack(44100, asset.NumChannels())

	// add samples
	track.AddClip(198450, asset.Clip(0, 44100))
	track.AddClip(66150, asset.Clip(44100, 44100))
	track.AddClip(132300, asset.Clip(0, 44100))

	// wav sink
	wavSink, err := wav.NewSink(
		test.Out.Example4,
		signal.BitDepth16,
	)
	// portaudio sink
	paSink := portaudio.NewSink()

	// final pipe
	p, err := pipe.New(
		bufferSize,
		pipe.WithPump(track),
		pipe.WithSinks(wavSink, paSink),
	)
	check(err)

	err = pipe.Wait(p.Run())
	check(err)
}

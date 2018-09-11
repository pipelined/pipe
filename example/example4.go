package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/asset"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/track"
	"github.com/dudk/phono/wav"
)

// Example:
//		Read .wav file
// 		Split it to samples
// 		Put samples to track
//		Save track into .wav and play it with portaudio
func four() {
	bufferSize := phono.BufferSize(512)
	inPath := "../_testdata/sample1.wav"
	outPath := "../_testdata/out/example4.wav"

	// wav pump
	wavPump, err := wav.NewPump(inPath, bufferSize)
	check(err)

	// asset sink
	asset := &asset.Asset{
		SampleRate: wavPump.WavSampleRate(),
	}

	// import pipe
	importAsset := pipe.New(
		pipe.WithPump(wavPump),
		pipe.WithSinks(asset),
	)
	defer importAsset.Close()

	err = importAsset.Do(pipe.Run)
	check(err)

	// track pump
	track := track.New(bufferSize, asset.NumChannels())

	// add samples
	track.AddClip(198450, asset.Clip(0, 44100))
	track.AddClip(66150, asset.Clip(44100, 44100))
	track.AddClip(132300, asset.Clip(0, 44100))

	// wav sink
	wavSink, err := wav.NewSink(
		outPath,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
		wavPump.WavBitDepth(),
		wavPump.WavAudioFormat(),
	)
	// portaudio sink
	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)

	// final pipe
	p := pipe.New(
		pipe.WithPump(track),
		pipe.WithSinks(wavSink, paSink),
	)

	err = p.Do(pipe.Run)
}

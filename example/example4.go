package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/asset"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/portaudio"
	"github.com/dudk/phono/track"
	"github.com/dudk/phono/wav"
)

// Example 4:
//		Read .wav file
// 		Split it to samples
// 		Put samples to track
//		Save track into .wav and play it with portaudio
func four() {
	bufferSize := phono.BufferSize(512)
	inPath := "../_testdata/sample1.wav"
	outPath := "../_testdata/out/example4.wav"

	wavPump, err := wav.NewPump(inPath, bufferSize)
	check(err)
	asset := &asset.Sink{
		SampleRate: wavPump.WavSampleRate(),
	}
	importAsset := pipe.New(
		pipe.WithPump(wavPump),
		pipe.WithSinks(asset),
	)
	err = importAsset.Do(pipe.Run)
	check(err)

	track := track.New(bufferSize, asset.NumChannels())

	track.AddFrame(198450, asset.Frame(0, 44100))
	track.AddFrame(66150, asset.Frame(44100, 44100))
	track.AddFrame(132300, asset.Frame(0, 44100))

	wavSink, err := wav.NewSink(
		outPath,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
		wavPump.WavBitDepth(),
		wavPump.WavAudioFormat(),
	)
	paSink := portaudio.NewSink(
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)

	p := pipe.New(
		pipe.WithPump(track),
		pipe.WithSinks(wavSink, paSink),
	)

	err = p.Do(pipe.Run)
}

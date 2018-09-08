package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/vst2"
	"github.com/dudk/phono/wav"
	vst2sdk "github.com/dudk/vst2"
)

// Example:
//		Read .wav file
//		Process it with VST2 plugin
// 		Save result into new .wav file
func two() {
	inPath := "../_testdata/sample1.wav"
	outPath := "../_testdata/out/example2.wav"
	bufferSize := phono.BufferSize(512)
	wavPump, err := wav.NewPump(
		inPath,
		bufferSize,
	)
	check(err)

	vst2path := "../_testdata/Krush.vst"
	vst2lib, err := vst2sdk.Open(vst2path)
	check(err)
	defer vst2lib.Close()

	vst2plugin, err := vst2lib.Open()
	check(err)
	defer vst2plugin.Close()
	vst2processor := vst2.NewProcessor(
		vst2plugin,
		bufferSize,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
	)
	wavSink, err := wav.NewSink(
		outPath,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
		wavPump.WavBitDepth(),
		wavPump.WavAudioFormat(),
	)
	check(err)
	p := pipe.New(
		pipe.WithPump(wavPump),
		pipe.WithProcessors(vst2processor),
		pipe.WithSinks(wavSink),
	)
	defer p.Close()
	err = p.Do(pipe.Run)
	check(err)
}

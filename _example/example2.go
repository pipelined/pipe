package example

import (
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/vst2"
	"github.com/pipelined/phono/wav"
	vst2sdk "github.com/pipelined/vst2"
)

// Example:
//		Read .wav file
//		Process it with VST2 plugin
// 		Save result into new .wav file
func two() {
	bufferSize := 512
	wavPump, err := wav.NewPump(
		test.Data.Wav1,
		bufferSize,
	)
	check(err)
	sampleRate := wavPump.SampleRate()

	vst2lib, err := vst2sdk.Open(test.Vst)
	check(err)
	defer vst2lib.Close()

	vst2plugin, err := vst2lib.Open()
	check(err)
	defer vst2plugin.Close()
	vst2processor := vst2.NewProcessor(
		vst2plugin,
		bufferSize,
		wavPump.SampleRate(),
		wavPump.NumChannels(),
	)
	wavSink, err := wav.NewSink(
		test.Out.Example2,
		wavPump.SampleRate(),
		wavPump.NumChannels(),
		wavPump.BitDepth(),
		wavPump.Format(),
	)
	check(err)
	p, err := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump),
		pipe.WithProcessors(vst2processor),
		pipe.WithSinks(wavSink),
	)
	check(err)
	defer p.Close()
	err = pipe.Wait(p.Run())
	check(err)
}

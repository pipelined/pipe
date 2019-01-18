package example

import (
	"github.com/pipelined/phono/mixer"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/vst2"
	"github.com/pipelined/phono/wav"
	vst2sdk "github.com/pipelined/vst2"
)

// Example:
//		Read two .wav files
//		Mix them
// 		Process with vst2
//		Save result into new .wav file
//
// NOTE: For example both wav files have same characteristics i.e: sample rate, bit depth and number of channels.
// In real life implicit conversion will be needed.
func five() {
	bs := 512

	// wav pump 1
	wavPump1, err := wav.NewPump(test.Data.Wav1, bs)
	check(err)

	// wav pump 2
	wavPump2, err := wav.NewPump(test.Data.Wav2, bs)
	check(err)

	sampleRate := wavPump1.SampleRate()
	// mixer
	mixer := mixer.New(bs, wavPump1.NumChannels())

	// track 1
	track1, err := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump1),
		pipe.WithSinks(mixer),
	)
	check(err)
	defer track1.Close()
	// track 2
	track2, err := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump2),
		pipe.WithSinks(mixer),
	)
	check(err)
	defer track2.Close()

	// vst2 processor
	vst2lib, err := vst2sdk.Open(test.Vst)
	check(err)
	defer vst2lib.Close()

	vst2plugin, err := vst2lib.Open()
	check(err)
	defer vst2plugin.Close()

	vst2processor := vst2.NewProcessor(
		vst2plugin,
		bs,
		wavPump1.SampleRate(),
		wavPump1.NumChannels(),
	)

	// wav sink
	wavSink, err := wav.NewSink(
		test.Out.Example5,
		wavPump1.SampleRate(),
		wavPump1.NumChannels(),
		wavPump1.BitDepth(),
		wavPump1.Format(),
	)
	check(err)

	// out pipe
	out, err := pipe.New(
		sampleRate,
		pipe.WithPump(mixer),
		pipe.WithProcessors(vst2processor),
		pipe.WithSinks(wavSink),
	)
	check(err)
	defer out.Close()

	track1Errc := track1.Run()
	check(err)
	track2Errc := track2.Run()
	check(err)
	outErrc := out.Run()
	check(err)

	err = pipe.Wait(track1Errc)
	check(err)
	err = pipe.Wait(track2Errc)
	check(err)
	err = pipe.Wait(outErrc)
	check(err)
}

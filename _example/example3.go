package example

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/mixer"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/test"
	"github.com/dudk/phono/wav"
)

// Example:
//		Read two .wav files
//		Mix them
//		Save result into new .wav file
//
// NOTE: For example both wav files have same characteristics i.e: sample rate, bit depth and number of channels.
// In real life implicit conversion will be needed.
func three() {
	bs := phono.BufferSize(512)

	wavPump1, err := wav.NewPump(test.Data.Wav1, bs)
	check(err)
	wavPump2, err := wav.NewPump(test.Data.Wav2, bs)
	check(err)
	sampleRate := wavPump1.WavSampleRate()

	wavSink, err := wav.NewSink(
		test.Out.Example3,
		wavPump1.WavSampleRate(),
		wavPump1.WavNumChannels(),
		wavPump1.WavBitDepth(),
		wavPump1.WavAudioFormat(),
	)
	check(err)
	mixer := mixer.New(bs, wavPump1.WavNumChannels())

	track1 := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump1),
		pipe.WithSinks(mixer),
	)
	defer track1.Close()
	track2 := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump2),
		pipe.WithSinks(mixer),
	)
	defer track2.Close()
	out := pipe.New(
		sampleRate,
		pipe.WithPump(mixer),
		pipe.WithSinks(wavSink),
	)
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

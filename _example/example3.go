package example

import (
	"github.com/pipelined/phono/mixer"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/signal"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
)

// Example:
//		Read two .wav files
//		Mix them
//		Save result into new .wav file
//
// NOTE: For example both wav files have same characteristics i.e: sample rate, bit depth and number of channels.
// In real life implicit conversion will be needed.
func three() {
	bufferSize := 512

	wavPump1 := wav.NewPump(test.Data.Wav1)

	wavPump2 := wav.NewPump(test.Data.Wav2)

	wavSink, err := wav.NewSink(
		test.Out.Example3,
		signal.BitDepth16,
	)
	check(err)
	mixer := mixer.New()

	track1, err := pipe.New(
		bufferSize,
		pipe.WithPump(wavPump1),
		pipe.WithSinks(mixer),
	)
	check(err)
	defer track1.Close()
	track2, err := pipe.New(
		bufferSize,
		pipe.WithPump(wavPump2),
		pipe.WithSinks(mixer),
	)
	check(err)
	defer track2.Close()
	out, err := pipe.New(
		bufferSize,
		pipe.WithPump(mixer),
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

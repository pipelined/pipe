package mixer_test

import (
	"testing"

	"github.com/dudk/mixer"
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"

	"github.com/dudk/wav"
)

const (
	wavPath1 = "_testdata/test1.wav"
	wavPath2 = "_testdata/test2.wav"
	outPath  = "_testdata/out.wav"
)

func TestMixer(t *testing.T) {
	bs := phono.BufferSize(512)

	p1, _ := wav.NewPump(wavPath1, bs)
	p2, _ := wav.NewPump(wavPath2, bs)

	s := wav.NewSink(outPath, p1.WavSampleRate(), p1.WavNumChannels(), p1.WavBitDepth(), p1.WavAudioFormat())

	m := mixer.New(bs, p1.WavNumChannels())

	track1 := pipe.New(
		pipe.WithPump(p1),
		pipe.WithSinks(m),
	)
	track2 := pipe.New(
		pipe.WithPump(p2),
		pipe.WithSinks(m),
	)

	playback := pipe.New(
		pipe.WithPump(m),
		pipe.WithSinks(s),
	)

	pipe.Do(track1.Run)
	defer track1.Close()
	pipe.Do(track2.Run)
	defer track2.Close()
	pipe.Do(playback.Run)
	defer playback.Close()

	track1.Wait(pipe.Ready)
	track2.Wait(pipe.Ready)
	playback.Wait(pipe.Ready)

}

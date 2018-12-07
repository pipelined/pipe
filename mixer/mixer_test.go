package mixer_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mixer"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/test"
	"github.com/dudk/phono/wav"
)

var (
	bufferSize  = phono.BufferSize(10)
	numChannels = phono.NumChannels(1)
	tests       = []struct {
		mock.Limit
		value1   float64
		value2   float64
		sum      float64
		messages int64
		samples  int64
	}{
		{
			Limit:    3,
			value1:   0.5,
			value2:   0.7,
			sum:      0.6,
			messages: 3,
			samples:  30,
		},
		{
			Limit:    10,
			value1:   0.7,
			value2:   0.9,
			sum:      0.8,
			messages: 10,
			samples:  100,
		},
	}
)

func TestMixer(t *testing.T) {
	t.Skip("This test is broken")
	pump1 := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       1,
		BufferSize:  bufferSize,
		NumChannels: numChannels,
	}
	pump2 := &mock.Pump{
		UID:         phono.NewUID(),
		Limit:       1,
		BufferSize:  bufferSize,
		NumChannels: numChannels,
	}
	sampleRate := phono.SampleRate(44100)
	mix := mixer.New(bufferSize, numChannels)
	sink := &mock.Sink{UID: phono.NewUID()}
	playback, err := pipe.New(
		sampleRate,
		pipe.WithName("Playback"),
		pipe.WithPump(mix),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	track1, err := pipe.New(
		sampleRate,
		pipe.WithName("Track 1"),
		pipe.WithPump(pump1),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)
	track2, err := pipe.New(
		sampleRate,
		pipe.WithName("Track 2"),
		pipe.WithPump(pump2),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)

	for _, test := range tests {
		track1.Push(
			pump1.LimitParam(test.Limit),
			pump1.ValueParam(test.value1),
		)
		track2.Push(
			pump2.LimitParam(test.Limit),
			pump2.ValueParam(test.value2),
		)

		track1errc := track1.Run()
		assert.NotNil(t, track1errc)
		track2errc := track2.Run()
		assert.NotNil(t, track2errc)
		playbackerrc := playback.Run()
		assert.NotNil(t, playbackerrc)

		err := pipe.Wait(track1errc)
		assert.Nil(t, err)
		err = pipe.Wait(track2errc)
		assert.Nil(t, err)
		err = pipe.Wait(playbackerrc)
		assert.Nil(t, err)
		for i := range sink.Buffer {
			for _, val := range sink.Buffer[i] {
				assert.Equal(t, test.sum, val, fmt.Sprintf("Message: %v\n", i))
			}
		}
		messageCount, sampleCount := sink.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, sampleCount)
	}

	track1.Close()
	track2.Close()
	playback.Close()
}

func TestWavMixer(t *testing.T) {
	bs := phono.BufferSize(512)

	p1, _ := wav.NewPump(test.Data.Wav1, bs)
	p2, _ := wav.NewPump(test.Data.Wav2, bs)
	sampleRate := p1.WavSampleRate()

	s, _ := wav.NewSink(test.Out.Mixer, p1.WavSampleRate(), p1.WavNumChannels(), p1.WavBitDepth(), p1.WavAudioFormat())

	m := mixer.New(bs, p1.WavNumChannels())

	track1, err := pipe.New(
		sampleRate,
		pipe.WithPump(p1),
		pipe.WithSinks(m),
	)
	assert.Nil(t, err)
	track2, err := pipe.New(
		sampleRate,
		pipe.WithPump(p2),
		pipe.WithSinks(m),
	)
	assert.Nil(t, err)
	playback, err := pipe.New(
		sampleRate,
		pipe.WithPump(m),
		pipe.WithSinks(s),
	)
	assert.Nil(t, err)

	track1errc := track1.Run()
	assert.NotNil(t, track1errc)
	track2errc := track2.Run()
	assert.NotNil(t, track2errc)
	playbackerrc := playback.Run()
	assert.NotNil(t, playbackerrc)

	err = pipe.Wait(track1errc)
	assert.Nil(t, err)
	err = pipe.Wait(track2errc)
	assert.Nil(t, err)
	err = pipe.Wait(playbackerrc)
	assert.Nil(t, err)

	track1.Close()
	track2.Close()
	playback.Close()
}

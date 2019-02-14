package mixer_test

import (
	"fmt"
	"io"
	"testing"

	"go.uber.org/goleak"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/mixer"
	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/signal"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/wav"
)

func TestMixer(t *testing.T) {
	bufferSize := 10
	numChannels := 1
	tests := []struct {
		mock.Limit
		value1   float64
		value2   float64
		sum      float64
		messages int
		samples  int
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
	pump1 := &mock.Pump{
		Limit:       1,
		NumChannels: numChannels,
	}
	pump2 := &mock.Pump{
		Limit:       1,
		NumChannels: numChannels,
	}
	mix := mixer.New()
	sink := &mock.Sink{}
	track1, err := pipe.New(
		bufferSize,
		pipe.WithName("Track 1"),
		pipe.WithPump(pump1),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)
	track2, err := pipe.New(
		bufferSize,
		pipe.WithName("Track 2"),
		pipe.WithPump(pump2),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)
	playback, err := pipe.New(
		bufferSize,
		pipe.WithName("Playback"),
		pipe.WithPump(mix),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)

	for _, test := range tests {
		track1.Push(pump1,
			pump1.LimitParam(test.Limit),
			pump1.ValueParam(test.value1),
		)
		track2.Push(pump2,
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
		for i := range sink.Buffer() {
			for _, val := range sink.Buffer()[i] {
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
	bufferSize := 512

	p1 := wav.NewPump(test.Data.Wav1)
	p2 := wav.NewPump(test.Data.Wav2)

	s, err := wav.NewSink(test.Out.Mixer, signal.BitDepth16)

	m := mixer.New()

	track1, err := pipe.New(
		bufferSize,
		pipe.WithPump(p1),
		pipe.WithSinks(m),
	)
	assert.Nil(t, err)
	track2, err := pipe.New(
		bufferSize,
		pipe.WithPump(p2),
		pipe.WithSinks(m),
	)
	assert.Nil(t, err)
	playback, err := pipe.New(
		bufferSize,
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

func TestInterruptSink(t *testing.T) {
	bufferSize := 10
	pump := &mock.Pump{
		Limit:       10,
		NumChannels: 2,
		Interval:    100,
	}
	mix := mixer.New()
	sink := &mock.Sink{}
	playback, err := pipe.New(
		bufferSize,
		pipe.WithName("Playback"),
		pipe.WithPump(mix),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	track, err := pipe.New(
		bufferSize,
		pipe.WithName("Track 1"),
		pipe.WithPump(pump),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)

	track.Run()
	playback.Run()

	pipe.Wait(track.Pause())
	pipe.Wait(track.Close())
	err = pipe.Wait(playback.Close())
	assert.Nil(t, err)

	goleak.VerifyNoLeaks(t)
}

func TestInterruptPump(t *testing.T) {
	bufferSize := 10
	pump := &mock.Pump{
		Limit:       10,
		NumChannels: 2,
		Interval:    100,
	}
	mix := mixer.New()
	sink := &mock.Sink{}
	playback, err := pipe.New(
		bufferSize,
		pipe.WithName("Playback"),
		pipe.WithPump(mix),
		pipe.WithSinks(sink),
	)
	assert.Nil(t, err)
	track, err := pipe.New(
		bufferSize,
		pipe.WithName("Track 1"),
		pipe.WithPump(pump),
		pipe.WithSinks(mix),
	)
	assert.Nil(t, err)

	trackRun := track.Run()
	playback.Run()

	pipe.Wait(playback.Pause())
	err = pipe.Wait(playback.Close())
	assert.Nil(t, err)
	err = pipe.Wait(trackRun)
	assert.Equal(t, io.ErrClosedPipe, err)
	err = pipe.Wait(track.Close())
	assert.Nil(t, err)

	goleak.VerifyNoLeaks(t)
}

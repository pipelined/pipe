package track_test

import (
	"fmt"
	"testing"

	"github.com/dudk/phono/asset"
	"github.com/dudk/phono/mock"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/test"
	"github.com/dudk/phono/track"
	"github.com/dudk/phono/wav"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
)

var (
	bufferSize  = phono.BufferSize(512)
	sampleRate  = phono.SampleRate(44100)
	numChannels = phono.NumChannels(1)

	asset1 = &asset.Asset{
		UID:    phono.NewUID(),
		Buffer: [][]float64{[]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
	}
	asset2 = &asset.Asset{
		UID:    phono.NewUID(),
		Buffer: [][]float64{[]float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2}},
	}
	overlapTests = []struct {
		phono.BufferSize
		clips   []phono.Clip
		clipsAt []int64
		result  []float64
		msg     string
	}{
		{
			BufferSize: 2,
			clips: []phono.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt: []int64{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0},
			msg:     "Sequence",
		},
		{
			BufferSize: 3,
			clips: []phono.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt: []int64{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0, 0},
			msg:     "Sequence increased bufferSize",
		},
		{
			BufferSize: 2,
			clips: []phono.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt: []int64{2, 3},
			result:  []float64{0, 0, 1, 2, 2, 2},
			msg:     "Sequence shifted left",
		},
		{
			BufferSize: 2,
			clips: []phono.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt: []int64{2, 4},
			result:  []float64{0, 0, 1, 0, 2, 2, 2, 0},
			msg:     "Sequence with interval",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 3),
				asset2.Clip(5, 2),
			},
			clipsAt: []int64{3, 2},
			result:  []float64{0, 0, 2, 2, 1, 1},
			msg:     "Overlap previous",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 3),
				asset2.Clip(5, 2),
			},
			clipsAt: []int64{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2},
			msg:     "Overlap next",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 5),
				asset2.Clip(5, 2),
			},
			clipsAt: []int64{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap single in the middle",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(3, 2),
				asset2.Clip(5, 2),
			},
			clipsAt: []int64{2, 5, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap two in the middle",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(5, 2),
				asset2.Clip(3, 2),
			},
			clipsAt: []int64{2, 5, 3},
			result:  []float64{0, 0, 1, 2, 2, 1, 1, 0},
			msg:     "Overlap two in the middle shifted",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 2),
				asset2.Clip(3, 5),
			},
			clipsAt: []int64{2, 2},
			result:  []float64{0, 0, 2, 2, 2, 2, 2, 0},
			msg:     "Overlap single completely",
		},
		{
			clips: []phono.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(5, 2),
				asset2.Clip(1, 8),
			},
			clipsAt: []int64{2, 5, 1},
			result:  []float64{0, 2, 2, 2, 2, 2, 2, 2, 2, 0},
			msg:     "Overlap two completely",
		},
	}
)

func TestTrackWavSlices(t *testing.T) {
	wavPump, err := wav.NewPump(test.Data.Wav1, bufferSize)
	assert.Nil(t, err)
	asset := &asset.Asset{
		UID:        phono.NewUID(),
		SampleRate: sampleRate,
	}

	p1 := pipe.New(
		sampleRate,
		pipe.WithPump(wavPump),
		pipe.WithSinks(asset),
	)
	_ = p1.Do(pipe.Run)

	wavSink, err := wav.NewSink(
		test.Out.Track,
		wavPump.WavSampleRate(),
		wavPump.WavNumChannels(),
		wavPump.WavBitDepth(),
		wavPump.WavAudioFormat(),
	)
	track := track.New(bufferSize, asset.NumChannels())

	track.AddClip(198450, asset.Clip(0, 44100))
	track.AddClip(66150, asset.Clip(44100, 44100))
	track.AddClip(132300, asset.Clip(0, 44100))

	p2 := pipe.New(
		sampleRate,
		pipe.WithPump(track),
		pipe.WithSinks(wavSink),
	)
	_ = p2.Do(pipe.Run)
}

func TestSliceOverlaps(t *testing.T) {
	sink := &mock.Sink{UID: phono.NewUID()}
	bufferSize := phono.BufferSize(2)
	track := track.New(bufferSize, asset1.NumChannels())
	for _, test := range overlapTests {
		fmt.Printf("Starting: %v\n", test.msg)
		track.Reset()

		for i, clip := range test.clips {
			track.AddClip(test.clipsAt[i], clip)
		}

		p := pipe.New(
			sampleRate,
			pipe.WithPump(track),
			pipe.WithSinks(sink),
		)
		if test.BufferSize > 0 {
			p.Push(track.BufferSizeParam(test.BufferSize))
		}

		_ = p.Do(pipe.Run)
		assert.Equal(t, len(test.result), len(sink.Buffer[0]), test.msg)
		for i, v := range sink.Buffer[0] {
			assert.Equal(t, test.result[i], v, "Test: %v Index: %v Full expected: %v Full result:%v", test.msg, i, test.result, sink.Buffer[0])
		}
	}

}

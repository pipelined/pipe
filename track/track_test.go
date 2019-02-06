package track_test

import (
	"fmt"
	"testing"

	"github.com/pipelined/phono/asset"
	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/signal"
	"github.com/pipelined/phono/test"
	"github.com/pipelined/phono/track"
	"github.com/pipelined/phono/wav"

	"github.com/stretchr/testify/assert"
)

var (
	buffer1 = signal.Float64([][]float64{[]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	buffer2 = signal.Float64([][]float64{[]float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2}})

	overlapTests = []struct {
		clips   []signal.Float64Clip
		clipsAt []int
		result  []float64
		msg     string
	}{
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0},
			msg:     "Sequence",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0},
			msg:     "Sequence increased bufferSize",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{2, 3},
			result:  []float64{0, 0, 1, 2, 2, 2},
			msg:     "Sequence shifted left",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 0, 2, 2, 2, 0},
			msg:     "Sequence with interval",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 3),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{3, 2},
			result:  []float64{0, 0, 2, 2, 1, 1},
			msg:     "Overlap previous",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 3),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2},
			msg:     "Overlap next",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 5),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap single in the middle",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 2),
				buffer1.Clip(3, 2),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 5, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap two in the middle",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 2),
				buffer1.Clip(5, 2),
				buffer2.Clip(3, 2),
			},
			clipsAt: []int{2, 5, 3},
			result:  []float64{0, 0, 1, 2, 2, 1, 1, 0},
			msg:     "Overlap two in the middle shifted",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 2),
				buffer2.Clip(3, 5),
			},
			clipsAt: []int{2, 2},
			result:  []float64{0, 0, 2, 2, 2, 2, 2, 0},
			msg:     "Overlap single completely",
		},
		{
			clips: []signal.Float64Clip{
				buffer1.Clip(3, 2),
				buffer1.Clip(5, 2),
				buffer2.Clip(1, 8),
			},
			clipsAt: []int{2, 5, 1},
			result:  []float64{0, 2, 2, 2, 2, 2, 2, 2, 2, 0},
			msg:     "Overlap two completely",
		},
	}
)

func TestTrackWavSlices(t *testing.T) {
	bufferSize := 512
	wavPump := wav.NewPump(test.Data.Wav1)
	assetSink := &asset.Asset{}

	p1, err := pipe.New(
		bufferSize,
		pipe.WithPump(wavPump),
		pipe.WithSinks(assetSink),
	)
	assert.Nil(t, err)
	err = pipe.Wait(p1.Run())
	assert.Nil(t, err)

	wavSink, err := wav.NewSink(
		test.Out.Track,
		signal.BitDepth16,
	)
	asset := assetSink.Asset()
	track := track.New(44100, asset.NumChannels())

	track.AddClip(198450, asset.Clip(0, 44100))
	track.AddClip(66150, asset.Clip(44100, 44100))
	track.AddClip(132300, asset.Clip(0, 44100))

	p2, err := pipe.New(
		bufferSize,
		pipe.WithPump(track),
		pipe.WithSinks(wavSink),
	)
	assert.Nil(t, err)
	_ = pipe.Wait(p2.Run())
}

func TestSliceOverlaps(t *testing.T) {
	sink := &mock.Sink{}
	bufferSize := 2
	sampleRate := 44100
	track := track.New(sampleRate, buffer1.NumChannels())
	for _, test := range overlapTests {
		fmt.Printf("Starting: %v\n", test.msg)
		track.Reset()

		for i, clip := range test.clips {
			track.AddClip(test.clipsAt[i], clip)
		}

		p, err := pipe.New(
			bufferSize,
			pipe.WithPump(track),
			pipe.WithSinks(sink),
		)
		assert.Nil(t, err)

		_ = pipe.Wait(p.Run())
		assert.Equal(t, len(test.result), sink.Buffer().Size(), test.msg)
		for i, v := range sink.Buffer()[0] {
			assert.Equal(t, test.result[i], v, "Test: %v Index: %v Full expected: %v Full result:%v", test.msg, i, test.result, sink.Buffer()[0])
		}
	}

}

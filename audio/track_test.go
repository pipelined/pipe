package audio_test

import (
	"fmt"
	"testing"

	"github.com/pipelined/mock"
	"github.com/pipelined/phono/audio"
	"github.com/pipelined/phono/pipe"

	"github.com/stretchr/testify/assert"
)

var (
	buffer1 = audio.NewAsset([][]float64{[]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	buffer2 = audio.NewAsset([][]float64{[]float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2}})

	overlapTests = []struct {
		clips   []audio.Clip
		clipsAt []int
		result  []float64
		msg     string
	}{
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0},
			msg:     "Sequence",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{3, 4},
			result:  []float64{0, 0, 0, 1, 2, 2, 2, 0},
			msg:     "Sequence increased bufferSize",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{2, 3},
			result:  []float64{0, 0, 1, 2, 2, 2},
			msg:     "Sequence shifted left",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 1),
				buffer2.Clip(5, 3),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 0, 2, 2, 2, 0},
			msg:     "Sequence with interval",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 3),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{3, 2},
			result:  []float64{0, 0, 2, 2, 1, 1},
			msg:     "Overlap previous",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 3),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2},
			msg:     "Overlap next",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 5),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap single in the middle",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 2),
				buffer1.Clip(3, 2),
				buffer2.Clip(5, 2),
			},
			clipsAt: []int{2, 5, 4},
			result:  []float64{0, 0, 1, 1, 2, 2, 1, 0},
			msg:     "Overlap two in the middle",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 2),
				buffer1.Clip(5, 2),
				buffer2.Clip(3, 2),
			},
			clipsAt: []int{2, 5, 3},
			result:  []float64{0, 0, 1, 2, 2, 1, 1, 0},
			msg:     "Overlap two in the middle shifted",
		},
		{
			clips: []audio.Clip{
				buffer1.Clip(3, 2),
				buffer2.Clip(3, 5),
			},
			clipsAt: []int{2, 2},
			result:  []float64{0, 0, 2, 2, 2, 2, 2, 0},
			msg:     "Overlap single completely",
		},
		{
			clips: []audio.Clip{
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

func TestSliceOverlaps(t *testing.T) {
	sink := &mock.Sink{}
	bufferSize := 2
	sampleRate := 44100
	track := audio.NewTrack(sampleRate, buffer1.NumChannels())
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

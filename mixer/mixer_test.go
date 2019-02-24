package mixer_test

import (
	"io"
	"testing"

	"go.uber.org/goleak"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/mixer"
	"github.com/pipelined/signal"
)

type sinkConfig struct {
	messages  int
	value     float64
	interrupt bool
}

func TestNewMixer(t *testing.T) {
	tests := []struct {
		description string
		numChannels int
		bufferSize  int
		sampleRate  int
		tracks      []sinkConfig
		expected    [][]float64
		pumpLimit   int
	}{
		{
			description: "regular test",
			numChannels: 1,
			bufferSize:  2,
			sampleRate:  44100,
			tracks: []sinkConfig{
				{
					messages: 4,
					value:    0.7,
				},
				{
					messages: 3,
					value:    0.5,
				},
			},
			expected: [][]float64{{0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.7, 0.7}},
		},
		{
			description: "sink interrupt",
			numChannels: 1,
			bufferSize:  2,
			sampleRate:  44100,
			tracks: []sinkConfig{
				{
					messages: 4,
					value:    0.7,
				},
				{
					interrupt: true,
					messages:  3,
					value:     0.5,
				},
			},
			expected: [][]float64{{0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.7, 0.7}},
		},
		{
			description: "pump interrupt",
			numChannels: 1,
			bufferSize:  2,
			sampleRate:  44100,
			tracks: []sinkConfig{
				{
					messages: 4,
					value:    0.7,
				},
				{
					interrupt: true,
					messages:  3,
					value:     0.5,
				},
			},
			pumpLimit: 2,
			expected:  [][]float64{{0.6, 0.6, 0.6, 0.6}},
		},
	}

	var err error
	for _, test := range tests {
		t.Log(test.description)
		numTracks := len(test.tracks)
		pumpID := string(numTracks)
		mixer := mixer.New()
		// init sink funcs
		sinks := make([]func([][]float64) error, numTracks)
		for i := 0; i < numTracks; i++ {
			sinks[i], err = mixer.Sink(string(i), test.sampleRate, test.numChannels, test.bufferSize)
			assert.Nil(t, err)

		}
		// init pump func
		pump, _, _, err := mixer.Pump(pumpID, test.bufferSize)
		assert.Nil(t, err)

		// reset sinks
		for i := 0; i < numTracks; i++ {
			err = mixer.Reset(string(i))
			assert.Nil(t, err)
		}

		// reset pump
		err = mixer.Reset(string(pumpID))
		assert.Nil(t, err)

		// mixing cycle
		result := signal.Float64(make([][]float64, test.numChannels))
		var buffer [][]float64
		sent := 0
		for len(sinks) > 0 {
			i := 0
			for i < len(sinks) {
				// check if track is done
				if test.tracks[i].messages == sent {
					if test.tracks[i].interrupt {
						err = mixer.Interrupt(string(i))
						assert.Nil(t, err)
					} else {
						err = mixer.Flush(string(i))
						if err != nil {
							assert.Equal(t, io.ErrClosedPipe, err)
						}
					}

					sinks = append(sinks[:i], sinks[i+1:]...)
					test.tracks = append(test.tracks[:i], test.tracks[i+1:]...)
				} else {
					buf := buf(test.numChannels, test.bufferSize, test.tracks[i].value)
					err = sinks[i](buf)
					assert.Nil(t, err)
					i++
				}
			}
			buffer, err = pump()
			if buffer != nil {
				result = result.Append(buffer)
				assert.Nil(t, err)
			} else {
				assert.Equal(t, io.EOF, err)
			}
			sent++

			if test.pumpLimit > 0 && test.pumpLimit == sent {
				err = mixer.Interrupt(pumpID)
				assert.Nil(t, err)
				sinks = nil
			}
		}
		if test.pumpLimit == 0 {
			err = mixer.Flush(pumpID)
			assert.Nil(t, err)
		}
		goleak.VerifyNoLeaks(t)

		assert.Equal(t, len(test.expected), result.NumChannels(), "Incorrect result num channels")
		for i := range test.expected {
			assert.Equal(t, len(test.expected[i]), len(result[i]), "Incorrect result channel length")
			for j, val := range test.expected[i] {
				assert.Equal(t, val, result[i][j])
			}
		}
	}
}

func buf(numChannels, size int, value float64) [][]float64 {
	result := make([][]float64, numChannels)
	for i := range result {
		result[i] = make([]float64, size)
		for j := range result[i] {
			result[i][j] = value
		}
	}
	return result
}

func TestNewMixerInterrupt(t *testing.T) {

}

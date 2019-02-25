package audio_test

import (
	"testing"

	"github.com/pipelined/phono/audio"
	"github.com/pipelined/signal"
	"github.com/stretchr/testify/assert"
)

func TestNewAsset(t *testing.T) {
	tests := []struct {
		asset       *audio.Asset
		numChannels int
		value       float64
		messages    int
		samples     int
	}{
		{
			asset:       &audio.Asset{},
			numChannels: 1,
			value:       0.5,
			messages:    10,
			samples:     100,
		},
		{
			asset:       &audio.Asset{},
			numChannels: 2,
			value:       0.7,
			messages:    100,
			samples:     1000,
		},
		{
			asset:       &audio.Asset{},
			numChannels: 0,
			value:       0.7,
			messages:    0,
			samples:     0,
		},
		{
			asset:       nil,
			numChannels: 0,
			value:       0.7,
			messages:    0,
			samples:     0,
		},
	}
	bufferSize := 10

	for _, test := range tests {
		fn, err := test.asset.Sink("", 0, test.numChannels, bufferSize)
		assert.Nil(t, err)
		assert.NotNil(t, fn)
		for i := 0; i < test.messages; i++ {
			buf := signal.Mock(test.numChannels, bufferSize, test.value)
			err := fn(buf)
			assert.Nil(t, err)
		}
		assert.Equal(t, test.numChannels, test.asset.NumChannels())
		assert.Equal(t, test.samples, test.asset.Data().Size())
	}
}

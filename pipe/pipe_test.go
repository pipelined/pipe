package pipe

import (
	"context"
	"testing"

	"github.com/dudk/phono/pipe/vst2"
	"github.com/dudk/phono/pipe/wav"

	cache "github.com/dudk/phono/vst2"
	"github.com/stretchr/testify/assert"
)

var (
	inFile     = "../_testdata/test.wav"
	outFile    = "../_testdata/out.wav"
	outFile2   = "../_testdata/out2.wav"
	vstPath    = "../_testdata/vst2"
	vstName    = "Krush"
	bufferSize = 512
)

func TestPipe(t *testing.T) {
	cache := cache.NewCache(vstPath)
	defer cache.Close()
	plugin, err := cache.LoadPlugin(vstPath, vstName)
	assert.Nil(t, err)
	defer plugin.Close()
	wavPump, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	wavSink := wav.NewSink(
		outFile,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	wavSink1 := wav.NewSink(
		outFile2,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	vst2Processor := vst2.NewProcessor(*plugin)
	pipe := New(
		WithPump(wavPump),
		WithProcessors(vst2Processor),
		WithSinks(wavSink, wavSink1),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err = pipe.Run(ctx)
	assert.Nil(t, err)
}

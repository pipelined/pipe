package pipe

import (
	"context"
	"testing"

	"github.com/dudk/phono/pipe/processor"

	"github.com/dudk/phono/pipe/pump"
	"github.com/dudk/phono/pipe/sink"
	"github.com/dudk/phono/vst2"
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
	cache := vst2.NewCache(vstPath)
	defer cache.Close()
	plugin, err := cache.LoadPlugin(vstPath, vstName)
	assert.Nil(t, err)
	defer plugin.Close()
	wavPump, err := pump.NewWav(inFile, bufferSize)
	assert.Nil(t, err)
	wavSink := sink.NewWav(
		outFile,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	wavSink1 := sink.NewWav(
		outFile2,
		bufferSize,
		wavPump.SampleRate,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	vst2Processor := processor.NewVst2(plugin)
	pipe := New(
		Pump(wavPump),
		Processors(vst2Processor),
		Sinks(wavSink, wavSink1),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err = pipe.Run(ctx)
	assert.Nil(t, err)
}

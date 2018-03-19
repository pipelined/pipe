package pipe

import (
	"context"
	"testing"

	"github.com/dudk/phono/session"
	"github.com/dudk/phono/vst2"
	"github.com/dudk/phono/wav"

	"github.com/dudk/phono/cache"
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
	cache := cache.NewVST2(vstPath)
	defer cache.Close()
	plugin, err := cache.LoadPlugin(vstPath, vstName)
	assert.Nil(t, err)
	defer plugin.Close()
	wavPump, err := wav.NewPump(inFile, bufferSize)
	assert.Nil(t, err)
	session := session.New(
		session.BufferSize(bufferSize),
		session.NumChannels(wavPump.NumChannels),
		session.SampleRate(wavPump.SampleRate),
	)
	wavSink := wav.NewSink(
		outFile,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	wavSink1 := wav.NewSink(
		outFile2,
		wavPump.BitDepth,
		wavPump.NumChannels,
		wavPump.WavAudioFormat,
	)
	vst2Processor := vst2.NewProcessor(plugin)
	pipe := New(
		*session,
		WithPump(wavPump),
		WithProcessors(vst2Processor),
		WithSinks(wavSink, wavSink1),
	)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	err = pipe.Run(ctx)
	assert.Nil(t, err)
}

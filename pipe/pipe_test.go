package pipe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono/pipe"
)

var (
	bufferSize = 512
)

// TODO: build tests with mock package

func TestPipe(t *testing.T) {

	p := pipe.New(
		pipe.WithPump(nil),
		pipe.WithProcessors(nil),
	)
	err := p.Validate()
	assert.Nil(t, err)

	// assert.Equal(t, bufferSize, 512)
	// cache := cache.NewVST2(vstPath)
	// defer cache.Close()
	// plugin, err := cache.LoadPlugin(vstPath, vstName)
	// assert.Nil(t, err)
	// defer plugin.Close()
	// wavPump, err := wav.NewPump(inFile, bufferSize)
	// assert.Nil(t, err)
	// session := session.New(
	// 	session.BufferSize(bufferSize),
	// 	session.NumChannels(wavPump.NumChannels),
	// 	session.SampleRate(wavPump.SampleRate),
	// )
	// wavSink := wav.NewSink(
	// 	outFile,
	// 	wavPump.BitDepth,
	// 	wavPump.WavAudioFormat,
	// )
	// wavSink1 := wav.NewSink(
	// 	outFile2,
	// 	wavPump.BitDepth,
	// 	wavPump.WavAudioFormat,
	// )
	// vst2Processor := vst2.NewProcessor(plugin)
	// pipe := New(
	// 	*session,
	// 	WithPump(wavPump),
	// 	WithProcessors(vst2Processor),
	// 	WithSinks(wavSink, wavSink1),
	// )
	// ctx, cancelFunc := context.WithCancel(context.Background())
	// defer cancelFunc()
	// pc := make(chan phono.Pulse)
	// defer close(pc)

	// err = pipe.Run(ctx, session.Pulse(), pc)
	// assert.Nil(t, err)
}

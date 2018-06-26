# Phono
Golang audio pipe line

### Mock package
The purpose of this package is to provide useful pipe mocks to test pumps, processors and sinks.

#### mock.Pump example

TBD

#### mock.Processor example
[Wav package](https://github.com/dudk/phono/wav) contains Pump and Sink which allows to read/write wav files respectively. To test both components next tests structure were defined:
```go
var tests = []struct {
	phono.BufferSize
	inFile   string
	outFile  string
	messages uint64
	samples  uint64
}{
	{
		BufferSize: 512,
		inFile:     "_testdata/in.wav",
		outFile:    "_testdata/out.wav",
		messages:   uint64(646),
		samples:    uint64(330534),
	},
	{
		BufferSize: 512,
		inFile:     "_testdata/out.wav",
		outFile:    "_testdata/out1.wav",
		messages:   uint64(646),
		samples:    uint64(330534),
	},
}
```
And test function uses mock.Processor to validate information passed through pipe:
```go
func TestWavPipe(t *testing.T) {
	for _, test := range tests {
		pump, err := wav.NewPump(test.inFile, bufferSize)
		assert.Nil(t, err)
		sink := wav.NewSink(test.outFile, pump.WavSampleRate(), pump.WavNumChannels(), pump.WavBitDepth(), pump.WavAudioFormat())

		processor := &mock.Processor{}
		p := pipe.New(
			pipe.WithPump(pump),
			pipe.WithProcessors(processor),
			pipe.WithSinks(sink),
		)
		err = pipe.Do(p.Run)
		assert.Nil(t, err)
		err = p.Wait(pipe.Ready)
		assert.Nil(t, err)
		messageCount, sampleCount := processor.Count()
		assert.Equal(t, test.messages, messageCount)
		assert.Equal(t, test.samples, sampleCount)
	}
}
```
#### mock.Sink example

TBD

# WAV phono implementation
WAV pump and sink

# Usage
## Pump
```go
inFile := "test.wav"
bufferSize := 512
wavPump, err := wav.NewPump(inFile, bufferSize)
```

## Sink
```go
bitDepth := 24
outFile := "out.wav"
wavAudioFormat := 1
wavSink := wav.NewSink(
	outFile,
	bitDepth,
	wavAudioFormat,
)
```

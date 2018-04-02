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
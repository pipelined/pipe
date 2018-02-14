package sink

import (
	"context"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

//Wav sink saves audio to wav file
type Wav struct {
	Path           string
	BufferSize     int
	SampleRate     int
	BitDepth       int
	NumChannels    int
	WavAudioFormat int
}

//NewWav creates new wav sink
func NewWav(path string, bufferSize int, sampleRate int, bitDepth int, numChannels int, wavAudioFormat int) *Wav {
	return &Wav{
		Path:           path,
		BufferSize:     bufferSize,
		SampleRate:     sampleRate,
		BitDepth:       bitDepth,
		NumChannels:    numChannels,
		WavAudioFormat: wavAudioFormat,
	}
}

//Sink implements Sinker interface
func (w Wav) Sink(ctx context.Context, in <-chan phono.Buffer) (<-chan error, error) {
	file, err := os.Create(w.Path)
	if err != nil {
		return nil, err
	}
	// setup the encoder and write all the frames
	e := wav.NewEncoder(file, w.SampleRate, w.BitDepth, w.NumChannels, int(w.WavAudioFormat))
	errc := make(chan error, 1)
	go func() {
		defer file.Close()
		defer close(errc)
		defer e.Close()
		for in != nil {
			select {
			case buffer, ok := <-in:
				if !ok {
					in = nil
				} else {
					buf := convertToWavBuffer(&buffer, w.SampleRate, w.BitDepth)
					if err := e.Write(&buf); err != nil {
						errc <- err
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return errc, nil
}

func convertToWavBuffer(buf *phono.Buffer, sampleRate int, bitDepth int) audio.IntBuffer {
	if buf == nil {
		return audio.IntBuffer{}
	}
	bufLen := len(buf.Samples) * len(buf.Samples[0])
	numChannels := len(buf.Samples)
	intBuffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
		Data:           make([]int, bufLen),
	}

	for i := range buf.Samples[0] {
		for j := range buf.Samples {
			intBuffer.Data[i*numChannels+j] = int(buf.Samples[j][i] * 0x7FFF)
		}
	}
	return intBuffer
}

package sink

import (
	"context"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	wav2 "github.com/go-audio/wav"
	wav "github.com/youpy/go-wav"
)

//Wav sink saves audio to wav file
type Wav struct {
	Path           string
	SampleRate     int
	BufferSize     int
	BitDepth       int
	NumChannels    int
	WavAudioFormat int
}

//Sink implements Sinker interface
func (w *Wav) Sink(ctx context.Context, in <-chan phono.Buffer) (errc chan error, err error) {
	file, err := os.Create(w.Path)
	if err != nil {
		return nil, err
	}
	errc = make(chan error, 1)
	go func() {
		defer file.Close()
		defer close(errc)
		samples := make([]wav.Sample, 0, 300000)

		numSamples := 0
		for in != nil {
			select {
			case buffer, ok := <-in:
				if !ok {
					in = nil
				} else {
					numSamples += buffer.Size()
					newSamples := convertFloat64ToWavSamples(buffer.Samples)
					samples = append(samples, newSamples...)
				}
			case <-ctx.Done():
				return
			}
		}

		writer := wav.NewWriter(file, uint32(numSamples), 2, uint32(w.SampleRate), uint16(w.BitDepth))
		err := writer.WriteSamples(samples)
		if err != nil {
			errc <- err
			return
		}

	}()

	return errc, nil
}

func convertFloat64ToWavSamples(samples [][]float64) (wavSamples []wav.Sample) {
	wavSamples = make([]wav.Sample, len(samples[0]))
	for i := 0; i < len(samples[0]); i++ {
		wavSamples[i].Values[0] = int(samples[0][i] * 0x7FFF)
		wavSamples[i].Values[1] = int(samples[1][i] * 0x7FFF)
	}
	return
}

//SinkNew implements Sinker interface
func (w *Wav) SinkNew(ctx context.Context, in <-chan phono.Buffer) (errc chan error, err error) {
	file, err := os.Create(w.Path)
	if err != nil {
		return nil, err
	}
	// setup the encoder and write all the frames
	e := wav2.NewEncoder(file, w.SampleRate, w.BitDepth, w.NumChannels, int(w.WavAudioFormat))
	errc = make(chan error, 1)
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
					if err := e.Write(buf); err != nil {
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

func convertToWavBuffer(buf *phono.Buffer, sampleRate int, bitDepth int) (intBuffer *audio.IntBuffer) {
	if buf == nil {
		return
	}
	bufLen := len(buf.Samples) * len(buf.Samples[0])
	numChannels := len(buf.Samples)
	intBuffer.Format = &audio.Format{
		NumChannels: numChannels,
		SampleRate:  sampleRate,
	}
	intBuffer.SourceBitDepth = bitDepth
	intBuffer.Data = make([]int, bufLen)

	for i := range buf.Samples[0] {
		for j := range buf.Samples {
			intBuffer.Data[i+j] = int(buf.Samples[j][i] * 0x7FFF)
		}
	}
	return
}

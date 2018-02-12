package sink

import (
	"context"
	"os"

	"github.com/dudk/phono/audio"
	wav "github.com/youpy/go-wav"
)

//Sinker is an interface for final stage in audio pipeline
type Sinker interface {
	Sink(ctx context.Context, in <-chan audio.Buffer) (errc <-chan error, err error)
}

//Wav sink saves audio to wav file
type Wav struct {
	Path       string
	SampleRate int
	BufferSize int
	BitDepth   int
}

//Sink implements Sinker interface
func (w *Wav) Sink(ctx context.Context, in <-chan audio.Buffer) (errc chan error, err error) {
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

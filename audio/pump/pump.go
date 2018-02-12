package pump

import (
	"context"
	"io"
	"os"

	"github.com/dudk/phono/audio"
	wav "github.com/youpy/go-wav"
)

//Pumper provides an interface for sources of samples
type Pumper interface {
	Pump(ctx context.Context) (out <-chan audio.Buffer, errc <-chan error, err error)
}

//Wav reads from wav file
type Wav struct {
	Path       string
	BufferSize int64
}

//Pump creates a new instance of wav-reader
func (wr *Wav) Pump(ctx context.Context) (<-chan audio.Buffer, <-chan error, error) {
	file, err := os.Open(wr.Path)
	if err != nil {
		return nil, nil, err
	}
	reader := wav.NewReader(file)
	format, err := reader.Format()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	numChannels := int(format.NumChannels)
	out := make(chan audio.Buffer)
	errc := make(chan error, 1)
	go func() {
		defer file.Close()
		defer close(out)
		defer close(errc)
		for {
			wavSamples, err := reader.ReadSamples(uint32(wr.BufferSize))
			if wavSamples != nil && len(wavSamples) > 0 {
				samples := convertWavSamplesToFloat64(wavSamples, numChannels)
				buffer := audio.Buffer{Samples: samples}
				select {
				case out <- buffer:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					return
				}
				errc <- err
			}

		}
	}()
	return out, errc, nil
}

//convert WAV samples to float64 slice
func convertWavSamplesToFloat64(wavSamples []wav.Sample, numChannels int) (samples [][]float64) {
	samples = make([][]float64, numChannels)

	for i := range samples {
		samples[i] = make([]float64, 0, len(wavSamples))
	}

	for _, wavSample := range wavSamples {
		for i := range samples {
			samples[i] = append(samples[i], float64(wavSample.Values[i])/0x8000)
		}
	}
	return samples
}

package pump

import (
	"context"
	"io"
	"os"

	"github.com/dudk/phono/audio"
	audio2 "github.com/go-audio/audio"
	wav2 "github.com/go-audio/wav"
	wav "github.com/youpy/go-wav"
)

//Wav reads from wav file
type Wav struct {
	Path        string
	BufferSize  int64
	NumChannels int
	BitDepth    int
}

//Pump starts the pump process
func (w *Wav) Pump(ctx context.Context) (<-chan audio.Buffer, <-chan error, error) {
	file, err := os.Open(w.Path)
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
			wavSamples, err := reader.ReadSamples(uint32(w.BufferSize))
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
				if err != io.EOF {
					errc <- err
				}
				return
			}

		}
	}()
	return out, errc, nil
}

//PumpNew starts the pump process
func (w *Wav) PumpNew(ctx context.Context) (<-chan audio.Buffer, <-chan error, error) {
	file, err := os.Open(w.Path)
	if err != nil {
		return nil, nil, err
	}
	decoder := wav2.NewDecoder(file)
	format := decoder.Format()

	w.BitDepth = int(decoder.BitDepth)
	w.NumChannels = format.NumChannels
	out := make(chan audio.Buffer)
	errc := make(chan error, 1)
	go func() {
		defer file.Close()
		defer close(out)
		defer close(errc)
		totalSamples := 0
		for {
			intBuf := audio2.IntBuffer{
				Format:         format,
				Data:           make([]int, w.BufferSize*2),
				SourceBitDepth: w.BitDepth,
			}
			readSamples, err := decoder.PCMBuffer(&intBuf)
			totalSamples = totalSamples + readSamples
			if readSamples == 0 {
				return
			}
			buffer := convertWavBuffer(&intBuf, readSamples)
			select {
			case out <- buffer:
			case <-ctx.Done():
				return
			}
			//fmt.Printf("Read samples: %v\n", readSamples)
			// if wavSamples != nil && len(wavSamples) > 0 {
			// 	samples := convertWavSamplesToFloat64(wavSamples, w.NumChannels)
			// 	buffer := audio.Buffer{Samples: samples}
			// 	select {
			// 	case out <- buffer:
			// 	case <-ctx.Done():
			// 		return
			// 	}
			// }
			if err != nil {
				if err != io.EOF {
					errc <- err
				}
				return
			}

		}
	}()
	return out, errc, nil
}

func convertWavBuffer(intBuffer *audio2.IntBuffer, wavLen int) (buf audio.Buffer) {
	if intBuffer == nil {
		return
	}
	numChannels := intBuffer.Format.NumChannels
	buf.Samples = make([][]float64, numChannels)

	size := wavLen / numChannels
	for i := range buf.Samples {
		buf.Samples[i] = make([]float64, 0, size)
		for j := i; j < wavLen; j = j + numChannels {
			buf.Samples[i] = append(buf.Samples[i], float64(intBuffer.Data[j])/0x8000)
		}
	}
	return
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

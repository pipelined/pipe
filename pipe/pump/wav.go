package pump

import (
	"context"
	"fmt"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

//Wav reads from wav file
type Wav struct {
	Path           string
	BufferSize     int
	NumChannels    int
	BitDepth       int
	SampleRate     int
	WavAudioFormat int
}

//NewWav creates a new wav pump and sets wav props
func NewWav(path string, bufferSize int) (*Wav, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("Wav is not valid")
	}

	return &Wav{
		Path:           path,
		BufferSize:     bufferSize,
		NumChannels:    decoder.Format().NumChannels,
		BitDepth:       int(decoder.BitDepth),
		SampleRate:     int(decoder.SampleRate),
		WavAudioFormat: int(decoder.WavAudioFormat),
	}, nil
}

//Pump starts the pump process
//once executed, wav attributes are accessible
func (w Wav) Pump(ctx context.Context) (<-chan phono.Message, <-chan error, error) {
	file, err := os.Open(w.Path)
	if err != nil {
		return nil, nil, err
	}
	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		file.Close()
		return nil, nil, fmt.Errorf("Wav is not valid")
	}
	format := decoder.Format()
	//w.BitDepth = int(decoder.BitDepth)
	//w.NumChannels = format.NumChannels
	//w.SampleRate = int(decoder.SampleRate)
	out := make(chan phono.Message)
	errc := make(chan error, 1)
	go func() {
		defer file.Close()
		defer close(out)
		defer close(errc)
		intBuf := audio.IntBuffer{
			Format:         format,
			Data:           make([]int, w.BufferSize*w.NumChannels),
			SourceBitDepth: w.BitDepth,
		}
		for {
			readSamples, err := decoder.PCMBuffer(&intBuf)
			if err != nil {
				errc <- err
				return
			}
			if readSamples == 0 {
				return
			}
			message := phono.Message{}
			message.PutSamples(convertFromWavBuffer(&intBuf, readSamples))
			//buffer := convertFromWavBuffer(&intBuf, readSamples)
			select {
			case out <- message:
			case <-ctx.Done():
				return
			}

		}
	}()
	return out, errc, nil
}

func convertFromWavBuffer(intBuffer *audio.IntBuffer, wavLen int) *phono.Buffer {
	if intBuffer == nil {
		return nil
	}
	buf := phono.Buffer{}
	numChannels := intBuffer.Format.NumChannels
	buf.Samples = make([][]float64, numChannels)

	size := wavLen / numChannels
	for i := range buf.Samples {
		buf.Samples[i] = make([]float64, 0, size)
		for j := i; j < wavLen; j = j + numChannels {
			buf.Samples[i] = append(buf.Samples[i], float64(intBuffer.Data[j])/0x8000)
		}
	}
	return &buf
}

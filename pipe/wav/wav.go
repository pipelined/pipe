package wav

import (
	"context"
	"fmt"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

//Pump reads from wav file
type Pump struct {
	Path           string
	BufferSize     int
	NumChannels    int
	BitDepth       int
	SampleRate     int
	WavAudioFormat int
}

//NewPump creates a new wav pump and sets wav props
func NewPump(path string, bufferSize int) (*Pump, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("Wav is not valid")
	}

	return &Pump{
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
func (w Pump) Pump(ctx context.Context) (<-chan phono.Message, <-chan error, error) {
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

//Sink sink saves audio to wav file
type Sink struct {
	Path           string
	BufferSize     int
	SampleRate     int
	BitDepth       int
	NumChannels    int
	WavAudioFormat int
}

//NewSink creates new wav sink
func NewSink(path string, bufferSize int, sampleRate int, bitDepth int, numChannels int, wavAudioFormat int) *Sink {
	return &Sink{
		Path:           path,
		BufferSize:     bufferSize,
		SampleRate:     sampleRate,
		BitDepth:       bitDepth,
		NumChannels:    numChannels,
		WavAudioFormat: wavAudioFormat,
	}
}

//Sink implements Sinker interface
func (w Sink) Sink(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
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
			case message, ok := <-in:
				if !ok {
					in = nil
				} else {
					buf := convertToWavBuffer(message.Samples(), w.SampleRate, w.BitDepth)
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

func convertToWavBuffer(buf *phono.Buffer, sampleRate int, bitDepth int) *audio.IntBuffer {
	if buf == nil {
		return nil
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
	return &intBuffer
}

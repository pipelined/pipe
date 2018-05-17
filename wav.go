package wav

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

type (
	// Pump reads from wav file
	// todo: implement conversion if needed
	Pump struct {
		filePath   string
		bufferSize phono.BufferSize
		newMessage phono.NewMessageFunc

		// properties of decoded wav
		wavNumChannels phono.NumChannels
		wavSampleRate  phono.SampleRate
		wavBitDepth    int
		wavAudioFormat int
		wavFormat      *audio.Format
	}

	// Sink sink saves audio to wav file
	Sink struct {
		filePath string

		wavSampleRate  phono.SampleRate
		wavNumChannels phono.NumChannels
		wavBitDepth    int
		wavAudioFormat int
	}
)

var (
	// ErrBufferSizeNotDefined is used when buffer size is not defined
	ErrBufferSizeNotDefined = errors.New("Buffer size is not defined")
	// ErrSampleRateNotDefined is used when buffer size is not defined
	ErrSampleRateNotDefined = errors.New("Sample rate is not defined")
	// ErrNumChannelsNotDefined is used when number of channels is not defined
	ErrNumChannelsNotDefined = errors.New("Number of channels is not defined")
)

// NewPump creates a new wav pump and sets wav props
func NewPump(path string, bufferSize phono.BufferSize) (*Pump, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, errors.New("Wav is not valid")
	}

	return &Pump{
		bufferSize:     bufferSize,
		filePath:       path,
		wavNumChannels: phono.NumChannels(decoder.Format().NumChannels),
		wavSampleRate:  phono.SampleRate(decoder.SampleRate),
		wavBitDepth:    int(decoder.BitDepth),
		wavAudioFormat: int(decoder.WavAudioFormat),
		wavFormat:      decoder.Format(),
	}, nil
}

// Pump starts the pump process
// once executed, wav attributes are accessible
func (p *Pump) Pump() phono.PumpFunc {
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan phono.Message, <-chan error, error) {
		file, err := os.Open(p.filePath)
		if err != nil {
			return nil, nil, err
		}
		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			file.Close()
			return nil, nil, fmt.Errorf("Wav is not valid")
		}
		out := make(chan phono.Message)
		errc := make(chan error, 1)
		go func() {
			defer file.Close()
			defer close(out)
			defer close(errc)
			// create new int buffer
			ib := p.newIntBuffer()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					readSamples, err := decoder.PCMBuffer(ib)
					if err != nil {
						errc <- err
						return
					}

					if readSamples == 0 {
						return
					}
					// p.position += phono.SamplePosition(readSamples)
					// prune buffer to actual size
					ib.Data = ib.Data[:readSamples]
					// convert buffer to samples
					samples, err := AsSamples(ib)
					if err != nil {
						errc <- err
						return
					}
					// create and send message
					message := newMessage()
					message.ApplyTo(p)
					message.Samples = samples
					out <- message
				}
			}
		}()
		return out, errc, nil
	}
}

// WavSampleRate returns wav's sample rate
func (p *Pump) WavSampleRate() phono.SampleRate {
	return p.wavSampleRate
}

// WavNumChannels returns wav's number of channels
func (p *Pump) WavNumChannels() phono.NumChannels {
	return p.wavNumChannels
}

// WavBitDepth returns wav's bit depth
func (p *Pump) WavBitDepth() int {
	return p.wavBitDepth
}

// WavAudioFormat returns wav's audio format
func (p *Pump) WavAudioFormat() int {
	return p.wavAudioFormat
}

func (p *Pump) newIntBuffer() *audio.IntBuffer {
	return &audio.IntBuffer{
		Format:         p.wavFormat,
		Data:           make([]int, int(p.bufferSize)*int(p.wavNumChannels)),
		SourceBitDepth: p.wavBitDepth,
	}
}

// NewSink creates new wav sink
func NewSink(path string, wavSampleRate phono.SampleRate, wavNumChannels phono.NumChannels, bitDepth int, wavAudioFormat int) *Sink {
	return &Sink{
		filePath:       path,
		wavSampleRate:  wavSampleRate,
		wavNumChannels: wavNumChannels,
		wavBitDepth:    bitDepth,
		wavAudioFormat: wavAudioFormat,
	}
}

// Sink implements Sink interface
func (s *Sink) Sink() phono.SinkFunc {
	return func(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
		file, err := os.Create(s.filePath)
		if err != nil {
			return nil, err
		}
		// setup the encoder and write all the frames
		e := wav.NewEncoder(file, int(s.wavSampleRate), s.wavBitDepth, int(s.wavNumChannels), s.wavAudioFormat)
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			defer file.Close()
			defer e.Close()
			ib := s.newIntBuffer()
			for in != nil {
				select {
				case message, ok := <-in:
					if !ok {
						in = nil
					} else {
						samples := message.Samples
						err := AsBuffer(ib, samples)
						if err = e.Write(ib); err != nil {
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
}

func (s *Sink) newIntBuffer() *audio.IntBuffer {
	return &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: int(s.wavNumChannels),
			SampleRate:  int(s.wavSampleRate),
		},
		SourceBitDepth: s.wavBitDepth,
	}
}

// AsSamples converts from audio.Buffer to [][]float64 samples
func AsSamples(b audio.Buffer) ([][]float64, error) {
	if b == nil {
		return nil, nil
	}

	if b.PCMFormat() == nil {
		return nil, errors.New("Format for Buffer is not defined")
	}

	numChannels := b.PCMFormat().NumChannels
	s := make([][]float64, numChannels)
	bufferLen := numChannels * b.NumFrames()

	switch b.(type) {
	case *audio.IntBuffer:
		ib := b.(*audio.IntBuffer)
		for i := range s {
			s[i] = make([]float64, 0, b.NumFrames())
			for j := i; j < bufferLen; j = j + numChannels {
				s[i] = append(s[i], float64(ib.Data[j])/0x8000)
			}
		}
		return s, nil
	default:
		return nil, fmt.Errorf("Conversion to [][]float64 from %T is not defined", b)
	}
}

// AsBuffer converts from [][]float64 to audio.Buffer
func AsBuffer(b audio.Buffer, s [][]float64) error {
	if b == nil || s == nil {
		return nil
	}

	numChannels := len(s)
	bufferLen := numChannels * len(s[0])
	switch b.(type) {
	case *audio.IntBuffer:
		ib := b.(*audio.IntBuffer)
		ib.Data = make([]int, bufferLen)
		for i := range s[0] {
			for j := range s {
				ib.Data[i*numChannels+j] = int(s[j][i] * 0x7fff)
			}
		}
		return nil
	default:
		return fmt.Errorf("Conversion to %T from [][]float64 is not defined", b)
	}
}

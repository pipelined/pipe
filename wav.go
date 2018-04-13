package wav

import (
	"context"
	"fmt"
	"os"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// Pump reads from wav file
type Pump struct {
	Path           string
	BufferSize     int
	NumChannels    int
	BitDepth       int
	SampleRate     int
	WavAudioFormat int
	Format         *audio.Format
	options        *phono.Options
	newMessage     phono.NewMessageFunc
}

// Sink sink saves audio to wav file
type Sink struct {
	Path           string
	BitDepth       int
	WavAudioFormat int
	SampleRate     int
	NumChannels    int
}

// NewPump creates a new wav pump and sets wav props
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
		NumChannels:    decoder.Format().NumChannels,
		BitDepth:       int(decoder.BitDepth),
		SampleRate:     int(decoder.SampleRate),
		WavAudioFormat: int(decoder.WavAudioFormat),
		Format:         decoder.Format(),
		BufferSize:     bufferSize,
	}, nil
}

// Pump starts the pump process
// once executed, wav attributes are accessible
func (p *Pump) Pump() (pumpFunc phono.PumpFunc) {
	pumpFunc = func(ctx context.Context, oc <-chan phono.Options) (<-chan phono.Message, <-chan error, error) {
		file, err := os.Open(p.Path)
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
				message := p.newMessage(p.options)
				message.Samples = samples

				select {
				case out <- message:
				case <-ctx.Done():
					return
				case options := <-oc:
					options.ApplyTo(p)
					p.options = &options
				}
			}
		}()
		return out, errc, nil
	}
	p.newMessage = pumpFunc.NewMessage()
	return pumpFunc
}

// Validate implements phono.OptionUser
func (p *Pump) Validate() error {
	// todo: validation
	return nil
}

func (p *Pump) newIntBuffer() *audio.IntBuffer {
	return &audio.IntBuffer{
		Format:         p.Format,
		Data:           make([]int, p.BufferSize*p.NumChannels),
		SourceBitDepth: p.BitDepth,
	}
}

// NewSink creates new wav sink
func NewSink(path string, bitDepth int, wavAudioFormat int) *Sink {
	return &Sink{
		Path:           path,
		BitDepth:       bitDepth,
		WavAudioFormat: wavAudioFormat,
	}
}

// Sink implements Sink interface
func (s *Sink) Sink() phono.SinkFunc {
	return func(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
		file, err := os.Create(s.Path)
		if err != nil {
			return nil, err
		}
		// setup the encoder and write all the frames
		e := wav.NewEncoder(file, s.SampleRate, s.BitDepth, s.NumChannels, int(s.WavAudioFormat))
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
						//TODO refactor
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

// Validate implements phono.OptionUser
func (s *Sink) Validate() error {
	// todo: validation
	return nil
}

func (s *Sink) newIntBuffer() *audio.IntBuffer {
	return &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: s.NumChannels,
			SampleRate:  s.SampleRate,
		},
		SourceBitDepth: s.BitDepth,
	}
}

// AsSamples converts from audio.Buffer to [][]float64 samples
func AsSamples(b audio.Buffer) ([][]float64, error) {
	if b == nil {
		return nil, nil
	}

	if b.PCMFormat() == nil {
		return nil, fmt.Errorf("Format for Buffer is not defined")
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

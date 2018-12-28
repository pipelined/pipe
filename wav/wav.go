package wav

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/dudk/phono"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

type (
	// Pump reads from wav file.
	Pump struct {
		phono.UID
		bufferSize phono.BufferSize

		// properties of decoded wav.
		wavNumChannels phono.NumChannels
		wavSampleRate  phono.SampleRate
		wavBitDepth    int
		wavAudioFormat int
		wavFormat      *audio.Format
		file           *os.File
		decoder        *wav.Decoder
		ib             *audio.IntBuffer
		// Once for single-use.
		once sync.Once
	}

	// Sink sink saves audio to wav file.
	Sink struct {
		phono.UID
		wavSampleRate  phono.SampleRate
		wavNumChannels phono.NumChannels
		wavBitDepth    int
		wavAudioFormat int
		file           *os.File
		encoder        *wav.Encoder
		ib             *audio.IntBuffer
	}
)

var (
	// ErrBufferSizeNotDefined is used when buffer size is not defined.
	ErrBufferSizeNotDefined = errors.New("Buffer size is not defined")
	// ErrSampleRateNotDefined is used when buffer size is not defined.
	ErrSampleRateNotDefined = errors.New("Sample rate is not defined")
	// ErrNumChannelsNotDefined is used when number of channels is not defined.
	ErrNumChannelsNotDefined = errors.New("Number of channels is not defined")
)

// NewPump creates a new wav pump and sets wav props.
func NewPump(path string, bufferSize phono.BufferSize) (*Pump, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		file.Close()
		return nil, errors.New("Wav is not valid")
	}

	return &Pump{
		UID:            phono.NewUID(),
		file:           file,
		decoder:        decoder,
		bufferSize:     bufferSize,
		wavNumChannels: phono.NumChannels(decoder.Format().NumChannels),
		wavSampleRate:  phono.SampleRate(decoder.SampleRate),
		wavBitDepth:    int(decoder.BitDepth),
		wavAudioFormat: int(decoder.WavAudioFormat),
		wavFormat:      decoder.Format(),
		ib: &audio.IntBuffer{
			Format:         decoder.Format(),
			Data:           make([]int, int(bufferSize)*decoder.Format().NumChannels),
			SourceBitDepth: int(decoder.BitDepth),
		},
	}, nil
}

// Flush closes the file.
func (p *Pump) Flush(string) error {
	return p.file.Close()
}

// Reset implements pipe.Resetter.
func (p *Pump) Reset(string) error {
	return phono.SingleUse(&p.once)
}

// Pump starts the pump process once executed, wav attributes are accessible.
func (p *Pump) Pump(string) (phono.PumpFunc, error) {
	return func() (phono.Buffer, error) {
		if p.decoder == nil {
			return nil, errors.New("Source is not defined")
		}

		readSamples, err := p.decoder.PCMBuffer(p.ib)
		if err != nil {
			return nil, err
		}

		if readSamples == 0 {
			return nil, phono.ErrEOP
		}
		// prune buffer to actual size
		p.ib.Data = p.ib.Data[:readSamples]
		// convert buffer to buffer
		b, err := AsSamples(p.ib)
		if err != nil {
			return nil, err
		}
		return b, nil
	}, nil
}

// WavSampleRate returns wav's sample rate.
func (p *Pump) WavSampleRate() phono.SampleRate {
	return p.wavSampleRate
}

// WavNumChannels returns wav's number of channels.
func (p *Pump) WavNumChannels() phono.NumChannels {
	return p.wavNumChannels
}

// WavBitDepth returns wav's bit depth.
func (p *Pump) WavBitDepth() int {
	return p.wavBitDepth
}

// WavAudioFormat returns wav's audio format.
func (p *Pump) WavAudioFormat() int {
	return p.wavAudioFormat
}

// NewSink creates new wav sink.
func NewSink(path string, wavSampleRate phono.SampleRate, wavNumChannels phono.NumChannels, bitDepth int, wavAudioFormat int) (*Sink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	e := wav.NewEncoder(f, int(wavSampleRate), bitDepth, int(wavNumChannels), wavAudioFormat)

	return &Sink{
		UID:            phono.NewUID(),
		file:           f,
		encoder:        e,
		wavSampleRate:  wavSampleRate,
		wavNumChannels: wavNumChannels,
		wavBitDepth:    bitDepth,
		wavAudioFormat: wavAudioFormat,
		ib: &audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: int(wavNumChannels),
				SampleRate:  int(wavSampleRate),
			},
			SourceBitDepth: bitDepth},
	}, nil
}

// Flush flushes encoder.
func (s *Sink) Flush(string) error {
	err := s.encoder.Close()
	if err != nil {
		return err
	}
	return s.file.Close()
}

// Sink returns new Sink function instance.
func (s *Sink) Sink(string) (phono.SinkFunc, error) {
	return func(b phono.Buffer) error {
		err := AsBuffer(b, s.ib)
		if err != nil {
			return err
		}
		return s.encoder.Write(s.ib)
	}, nil
}

// AsSamples converts from audio.Buffer to [][]float64 buffer.
func AsSamples(b audio.Buffer) (phono.Buffer, error) {
	if b == nil {
		return nil, nil
	}

	if b.PCMFormat() == nil {
		return nil, errors.New("Format for Buffer is not defined")
	}

	numChannels := b.PCMFormat().NumChannels
	s := phono.Buffer(make([][]float64, numChannels))
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

// AsBuffer converts from [][]float64 to audio.Buffer.
func AsBuffer(b phono.Buffer, ab audio.Buffer) error {
	if ab == nil || b == nil {
		return nil
	}

	switch ab.(type) {
	case *audio.IntBuffer:
		ib := ab.(*audio.IntBuffer)
		ib.Data = b.Ints()
		return nil
	default:
		return fmt.Errorf("Conversion to %T from [][]float64 is not defined", ab)
	}
}

package wav

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pipelined/phono"
)

type (
	// Pump reads from wav file.
	Pump struct {
		bufferSize int

		// properties of decoded wav.
		numChannels int
		sampleRate  int
		bitDepth    int
		format      int
		file        *os.File
		decoder     *wav.Decoder
		ib          *audio.IntBuffer
		// Once for single-use.
		once sync.Once
	}

	// Sink sink saves audio to wav file.
	Sink struct {
		sampleRate  int
		numChannels int
		bitDepth    int
		format      int
		file        *os.File
		encoder     *wav.Encoder
		ib          *audio.IntBuffer
	}
)

// NewPump creates a new wav pump and sets wav props.
func NewPump(path string, bufferSize int) (*Pump, error) {
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
		file:        file,
		decoder:     decoder,
		bufferSize:  bufferSize,
		numChannels: decoder.Format().NumChannels,
		sampleRate:  int(decoder.SampleRate),
		bitDepth:    int(decoder.BitDepth),
		format:      int(decoder.WavAudioFormat),
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
func (p *Pump) Pump(string) (func() (phono.Buffer, error), error) {
	return func() (phono.Buffer, error) {
		if p.decoder == nil {
			return nil, errors.New("Source is not defined")
		}

		readSamples, err := p.decoder.PCMBuffer(p.ib)
		if err != nil {
			return nil, err
		}

		if readSamples == 0 {
			return nil, io.EOF
		}
		// prune buffer to actual size
		p.ib.Data = p.ib.Data[:readSamples]
		b := phono.EmptyBuffer(p.numChannels, p.ib.NumFrames())
		b.ReadInts(p.ib.Data)
		if b.Size() != p.bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, nil
}

// SampleRate returns wav's sample rate.
func (p *Pump) SampleRate() int {
	return p.sampleRate
}

// NumChannels returns wav's number of channels.
func (p *Pump) NumChannels() int {
	return p.numChannels
}

// BitDepth returns wav's bit depth.
func (p *Pump) BitDepth() int {
	return p.bitDepth
}

// Format returns wav's value from format chunk.
func (p *Pump) Format() int {
	return p.format
}

// NewSink creates new wav sink.
func NewSink(path string, sampleRate int, numChannels int, bitDepth int, format int) (*Sink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	e := wav.NewEncoder(f, sampleRate, bitDepth, numChannels, format)

	return &Sink{
		file:        f,
		encoder:     e,
		sampleRate:  sampleRate,
		numChannels: numChannels,
		bitDepth:    bitDepth,
		format:      format,
		ib: &audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: numChannels,
				SampleRate:  sampleRate,
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
func (s *Sink) Sink(string) (func(phono.Buffer) error, error) {
	return func(b phono.Buffer) error {
		s.ib.Data = b.Ints()
		return s.encoder.Write(s.ib)
	}, nil
}

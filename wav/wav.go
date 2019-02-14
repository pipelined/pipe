package wav

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pipelined/signal"
)

type (
	// Pump reads from wav file.
	// This component cannot be reused for consequent runs.
	Pump struct {
		path    string
		file    *os.File
		decoder *wav.Decoder
	}

	// Sink sink saves audio to wav file.
	Sink struct {
		path     string
		bitDepth signal.BitDepth
		format   int
		file     *os.File
		encoder  *wav.Encoder
	}
)

// ErrUnsupportedBitDepth is returned when unsupported bit depth is used.
var ErrUnsupportedBitDepth = errors.New("Only 16 and 32 bit depth is supported")

// NewPump creates a new wav pump and sets wav props.
func NewPump(path string) *Pump {
	return &Pump{path: path}
}

// Flush closes the file.
func (p *Pump) Flush(string) error {
	return p.file.Close()
}

// Pump starts the pump process once executed, wav attributes are accessible.
func (p *Pump) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	file, err := os.Open(p.path)
	if err != nil {
		return nil, 0, 0, err
	}

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		err = file.Close()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("Wav is not valid, failed to close the file %v", p.path)
		}
		return nil, 0, 0, errors.New("Wav is not valid")
	}

	if signal.BitDepth(decoder.BitDepth) != signal.BitDepth16 && signal.BitDepth(decoder.BitDepth) != signal.BitDepth32 {
		return nil, 0, 0, ErrUnsupportedBitDepth
	}

	p.file = file
	p.decoder = decoder
	numChannels := decoder.Format().NumChannels
	sampleRate := int(decoder.SampleRate)
	bitDepth := int(decoder.BitDepth)

	ib := &audio.IntBuffer{
		Format:         decoder.Format(),
		Data:           make([]int, bufferSize*decoder.Format().NumChannels),
		SourceBitDepth: int(decoder.BitDepth),
	}

	return func() ([][]float64, error) {
		readSamples, err := p.decoder.PCMBuffer(ib)
		if err != nil {
			return nil, err
		}

		if readSamples == 0 {
			return nil, io.EOF
		}
		// prune buffer to actual size
		b := signal.InterInt{Data: ib.Data[:readSamples], NumChannels: numChannels, BitDepth: signal.BitDepth(bitDepth)}.AsFloat64()
		if b.Size() != bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, sampleRate, numChannels, nil
}

// NewSink creates new wav sink.
func NewSink(path string, bitDepth signal.BitDepth) (*Sink, error) {
	if bitDepth != signal.BitDepth16 && bitDepth != signal.BitDepth32 {
		return nil, ErrUnsupportedBitDepth
	}
	return &Sink{
		path:     path,
		bitDepth: bitDepth,
		format:   1,
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
func (s *Sink) Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	f, err := os.Create(s.path)
	if err != nil {
		return nil, err
	}
	e := wav.NewEncoder(f, sampleRate, int(s.bitDepth), numChannels, s.format)

	s.file = f
	s.encoder = e
	ib := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: int(s.bitDepth),
	}

	return func(b [][]float64) error {
		ib.Data = signal.Float64(b).AsInterInt(s.bitDepth)
		return s.encoder.Write(ib)
	}, nil
}

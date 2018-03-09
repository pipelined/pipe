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
		BufferSize:     bufferSize,
		NumChannels:    decoder.Format().NumChannels,
		BitDepth:       int(decoder.BitDepth),
		SampleRate:     int(decoder.SampleRate),
		WavAudioFormat: int(decoder.WavAudioFormat),
	}, nil
}

// Pump starts the pump process
// once executed, wav attributes are accessible
func (p *Pump) Pump(s phono.Session) phono.PumpFunc {
	return func(ctx context.Context) (<-chan phono.Message, <-chan error, error) {
		file, err := os.Open(p.Path)
		if err != nil {
			return nil, nil, err
		}
		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			file.Close()
			return nil, nil, fmt.Errorf("Wav is not valid")
		}
		format := decoder.Format()
		out := make(chan phono.Message)
		errc := make(chan error, 1)
		go func() {
			defer file.Close()
			defer close(out)
			defer close(errc)
			for {
				intBuf := &audio.IntBuffer{
					Format:         format,
					Data:           make([]int, p.BufferSize*p.NumChannels),
					SourceBitDepth: p.BitDepth,
				}
				readSamples, err := decoder.PCMBuffer(intBuf)
				if err != nil {
					errc <- err
					return
				}
				if readSamples == 0 {
					return
				}
				message := s.NewMessage()
				message.PutBuffer(intBuf, readSamples)
				select {
				case out <- message:
				case <-ctx.Done():
					return
				}

			}
		}()
		return out, errc, nil
	}
}

// Sink sink saves audio to wav file
type Sink struct {
	Path           string
	SampleRate     int
	BitDepth       int
	NumChannels    int
	WavAudioFormat int
}

// NewSink creates new wav sink
func NewSink(path string, bitDepth int, numChannels int, wavAudioFormat int) *Sink {
	return &Sink{
		Path:           path,
		BitDepth:       bitDepth,
		NumChannels:    numChannels,
		WavAudioFormat: wavAudioFormat,
	}
}

// Sink implements Sink interface
func (s *Sink) Sink(session phono.Session) phono.SinkFunc {
	return func(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
		file, err := os.Create(s.Path)
		if err != nil {
			return nil, err
		}
		// setup the encoder and write all the frames
		e := wav.NewEncoder(file, session.SampleRate(), s.BitDepth, s.NumChannels, int(s.WavAudioFormat))
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			defer file.Close()
			defer e.Close()
			for in != nil {
				select {
				case message, ok := <-in:
					if !ok {
						in = nil
					} else {
						buf := message.AsBuffer()
						if err := e.Write(buf.AsIntBuffer()); err != nil {
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

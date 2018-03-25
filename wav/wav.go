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
	NumChannels    int
	BitDepth       int
	SampleRate     int
	WavAudioFormat int
	Format         *audio.Format
	pulse          phono.Pulse
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
		// position:       0,
	}, nil
}

// Pump starts the pump process
// once executed, wav attributes are accessible
func (p *Pump) Pump(pulse phono.Pulse) phono.PumpFunc {
	p.pulse = pulse
	return func(ctx context.Context, pc <-chan phono.Pulse) (<-chan phono.Message, <-chan error, error) {
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
				samples, err := phono.AsSamples(ib)
				if err != nil {
					errc <- err
					return
				}
				// create and send message
				message := p.pulse.NewMessage()
				message.SetSamples(samples)
				select {
				case out <- message:
				case <-ctx.Done():
					return
				case pulse := <-pc:
					p.pulse = pulse
				}
			}
		}()
		return out, errc, nil
	}
}

func (p *Pump) newIntBuffer() *audio.IntBuffer {
	return &audio.IntBuffer{
		Format:         p.Format,
		Data:           make([]int, p.pulse.BufferSize()*p.pulse.NumChannels()),
		SourceBitDepth: p.BitDepth,
	}
}

// Sink sink saves audio to wav file
type Sink struct {
	Path           string
	BitDepth       int
	WavAudioFormat int
	pulse          phono.Pulse
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
func (s *Sink) Sink(pulse phono.Pulse) phono.SinkFunc {
	s.pulse = pulse
	return func(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
		file, err := os.Create(s.Path)
		if err != nil {
			return nil, err
		}
		// setup the encoder and write all the frames
		e := wav.NewEncoder(file, s.pulse.SampleRate(), s.BitDepth, s.pulse.NumChannels(), int(s.WavAudioFormat))
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
						samples := message.Samples()
						err := phono.AsBuffer(ib, samples)
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
			NumChannels: s.pulse.NumChannels(),
			SampleRate:  s.pulse.SampleRate(),
		},
		SourceBitDepth: s.BitDepth,
	}
}

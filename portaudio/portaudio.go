package portaudio

import (
	"github.com/dudk/phono"
	"github.com/gordonklaus/portaudio"
)

type (
	// Sink represets portaudio sink which allows to play audio using default device.
	Sink struct {
		phono.UID
		buf    []float32
		stream *portaudio.Stream
		sr     phono.SampleRate
		bs     phono.BufferSize
		nc     phono.NumChannels
	}
)

// NewSink returns new initialized sink which allows to play pipe.
func NewSink(bs phono.BufferSize, sr phono.SampleRate, nc phono.NumChannels) *Sink {
	return &Sink{
		UID: phono.NewUID(),
		bs:  bs,
		sr:  sr,
		nc:  nc,
	}
}

// Sink writes the buffer of data to portaudio stream.
// It aslo initilizes a portaudio api with default stream.
func (s *Sink) Sink(string) (phono.SinkFunc, error) {
	s.buf = make([]float32, int(s.bs)*int(s.nc))
	err := portaudio.Initialize()
	if err != nil {
		return nil, err
	}
	s.stream, err = portaudio.OpenDefaultStream(0, int(s.nc), float64(s.sr), int(s.bs), &s.buf)
	if err != nil {
		return nil, err
	}
	err = s.stream.Start()
	if err != nil {
		return nil, err
	}
	return func(b phono.Buffer) error {
		for i := range b[0] {
			for j := range b {
				s.buf[i*int(s.nc)+j] = float32(b[j][i])
			}
		}
		return s.stream.Write()
	}, nil
}

// Flush terminates portaudio structures.
func (s *Sink) Flush(string) error {
	err := s.stream.Stop()
	if err != nil {
		return err
	}
	err = s.stream.Close()
	if err != nil {
		return err
	}
	return portaudio.Terminate()
}

package portaudio

import (
	"github.com/gordonklaus/portaudio"
	"github.com/pipelined/phono"
)

type (
	// Sink represets portaudio sink which allows to play audio using default device.
	Sink struct {
		buf         []float32
		stream      *portaudio.Stream
		sampleRate  int
		bufferSize  int
		numChannels int
	}
)

// NewSink returns new initialized sink which allows to play pipe.
func NewSink(bufferSize int, sampleRate int, numChannels int) *Sink {
	return &Sink{
		bufferSize:  bufferSize,
		sampleRate:  sampleRate,
		numChannels: numChannels,
	}
}

// Sink writes the buffer of data to portaudio stream.
// It aslo initilizes a portaudio api with default stream.
func (s *Sink) Sink(string) (func(phono.Buffer) error, error) {
	s.buf = make([]float32, s.bufferSize*s.numChannels)
	err := portaudio.Initialize()
	if err != nil {
		return nil, err
	}
	s.stream, err = portaudio.OpenDefaultStream(0, s.numChannels, float64(s.sampleRate), s.bufferSize, &s.buf)
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
				s.buf[i*s.numChannels+j] = float32(b[j][i])
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

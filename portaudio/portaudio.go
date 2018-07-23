package portaudio

import (
	"github.com/dudk/phono"
	"github.com/dudk/phono/pipe"
	"github.com/dudk/phono/pipe/runner"
	"github.com/gordonklaus/portaudio"
)

type (
	// Sink represets portaudio sink which allows to play audio using default device
	Sink struct {
		phono.UID
		buf    []float32
		stream *portaudio.Stream
		sr     phono.SampleRate
		bs     phono.BufferSize
		nc     phono.NumChannels
	}
)

// NewSink returns new initialized sink which allows to play pipe
func NewSink(bs phono.BufferSize, sr phono.SampleRate, nc phono.NumChannels) *Sink {
	return &Sink{
		bs: bs,
		sr: sr,
		nc: nc,
	}
}

// RunSink returns new sink runner with defined configuration
// Before: initilize a portaudio api with default stream
// After: flushe data and clean up portaudio api
func (s *Sink) RunSink(sourceID string) pipe.SinkRunner {
	return &runner.Sink{
		Sink: s,
		Before: func() (err error) {
			s.buf = make([]float32, int(s.bs)*int(s.nc))
			err = portaudio.Initialize()
			if err != nil {
				return
			}
			s.stream, err = portaudio.OpenDefaultStream(0, int(s.nc), float64(s.sr), int(s.bs), &s.buf)
			if err != nil {
				return
			}
			err = s.stream.Start()
			if err != nil {
				return
			}
			return
		},
		After: func() (err error) {
			err = s.stream.Stop()
			if err != nil {
				return
			}
			err = s.stream.Close()
			if err != nil {
				return
			}
			return portaudio.Terminate()
		},
	}
}

// Sink writes the buffer of data to portaudio stream
func (s *Sink) Sink(m *phono.Message) error {
	for i := range m.Buffer[0] {
		for j := range m.Buffer {
			s.buf[i*int(s.nc)+j] = float32(m.Buffer[j][i])
		}
	}
	return s.stream.Write()
}

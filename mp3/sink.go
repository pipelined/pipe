package mp3

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"

	"github.com/pipelined/phono"
	"github.com/viert/lame"
)

// Sink allows to send data to mp3 files.
type Sink struct {
	f    *os.File
	wr   *lame.LameWriter
	once sync.Once
}

// NewSink creates new Sink.
func NewSink(path string, sampleRate int, numChannels int, bitRate int, quality int) (*Sink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	s := Sink{
		f:  f,
		wr: lame.NewWriter(f),
	}
	s.wr.Encoder.SetBitrate(bitRate)
	s.wr.Encoder.SetQuality(quality)
	s.wr.Encoder.SetNumChannels(int(numChannels))
	s.wr.Encoder.SetInSamplerate(int(sampleRate))
	s.wr.Encoder.SetMode(lame.JOINT_STEREO)
	s.wr.Encoder.SetVBR(lame.VBR_RH)
	s.wr.Encoder.InitParams()
	return &s, nil
}

// Reset is used to prevent additional runs of the sink.
func (s *Sink) Reset(string) error {
	return phono.SingleUse(&s.once)
}

// Flush cleans up buffers.
func (s *Sink) Flush(string) error {
	err := s.wr.Close()
	if err != nil {
		return err
	}

	return s.f.Close()
}

// Sink writes buffer into file.
func (s *Sink) Sink(string) (func(phono.Buffer) error, error) {
	return func(b phono.Buffer) error {
		buf := new(bytes.Buffer)
		ints := b.Ints()
		for i := range ints {
			if err := binary.Write(buf, binary.LittleEndian, int16(ints[i])); err != nil {
				return err
			}
		}
		if _, err := s.wr.Write(buf.Bytes()); err != nil {
			return err
		}

		return nil
	}, nil
}

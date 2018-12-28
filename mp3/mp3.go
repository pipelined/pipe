package mp3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/dudk/phono"
	"github.com/viert/lame"
)

// Sink allows to send data to mp3 files.
type Sink struct {
	phono.UID
	f    *os.File
	wr   *lame.LameWriter
	once sync.Once
}

// NewSink creates new Sink.
func NewSink(path string, sampleRate phono.SampleRate, numChannels phono.NumChannels, bitRate int, quality int) (*Sink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	s := Sink{
		UID: phono.NewUID(),
		f:   f,
		wr:  lame.NewWriter(f),
	}
	s.wr.Encoder.SetBitrate(bitRate)
	s.wr.Encoder.SetQuality(quality)
	s.wr.Encoder.SetNumChannels(int(numChannels))
	s.wr.Encoder.SetInSamplerate(int(sampleRate))
	s.wr.Encoder.InitParams()
	s.wr.Encoder.SetMode(lame.JOINT_STEREO)
	s.wr.Encoder.SetVBR(lame.VBR_RH)
	fmt.Printf("Bitrate: %v Quality: %v NumChannels: %v InSampleRate: %v\n", bitRate, quality, numChannels, sampleRate)
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
func (s *Sink) Sink(string) (phono.SinkFunc, error) {
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

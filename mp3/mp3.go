package mp3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/viert/lame"

	"github.com/pipelined/signal"

	mp3 "github.com/hajimehoshi/go-mp3"
)

// Pump allows to read mp3 files.
// This component cannot be reused for consequent runs.
type Pump struct {
	path string
	f    *os.File
	d    *mp3.Decoder
}

// NewPump creates new mp3 Pump.
func NewPump(path string) *Pump {
	return &Pump{
		path: path,
	}
}

// Pump reads buffer from mp3.
func (p *Pump) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	var err error

	p.f, err = os.Open(p.path)
	if err != nil {
		return nil, 0, 0, err
	}

	p.d, err = mp3.NewDecoder(p.f)
	if err != nil {
		errClose := p.f.Close()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("Failed to close file %v: %v caused by %v", p.path, errClose, err)
		}
		return nil, 0, 0, err
	}

	// current decoder always provides stereo, so constant
	numChannels := 2
	sampleRate := p.d.SampleRate()

	return func() ([][]float64, error) {
		capacity := bufferSize * numChannels
		ints := make([]int, 0, capacity)

		var val int16
		for len(ints) < capacity {
			if err := binary.Read(p.d, binary.LittleEndian, &val); err != nil { // read next frame
				if err == io.EOF { // no bytes available
					if len(ints) == 0 {
						return nil, io.EOF
					}
					break
				} else { // error happened
					return nil, err
				}
			} else {
				ints = append(ints, int(val)) // append data
			}
		}

		b := signal.InterInt{Data: ints, NumChannels: numChannels, BitDepth: signal.BitDepth16}.AsFloat64()
		// read not enough samples
		if b.Size() != bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, sampleRate, numChannels, nil
}

// Flush all buffers.
func (p *Pump) Flush(string) error {
	return p.d.Close()
}

// Sink allows to send data to mp3 files.
type Sink struct {
	path    string
	bitRate int
	quality int
	f       *os.File
	wr      *lame.LameWriter
}

// NewSink creates new Sink.
func NewSink(path string, bitRate int, quality int) *Sink {
	s := Sink{
		path:    path,
		bitRate: bitRate,
		quality: quality,
	}
	return &s
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
func (s *Sink) Sink(sourceID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	var err error
	s.f, err = os.Create(s.path)
	if err != nil {
		return nil, err
	}

	s.wr = lame.NewWriter(s.f)
	s.wr.Encoder.SetBitrate(s.bitRate)
	s.wr.Encoder.SetQuality(s.quality)
	s.wr.Encoder.SetNumChannels(int(numChannels))
	s.wr.Encoder.SetInSamplerate(int(sampleRate))
	s.wr.Encoder.SetMode(lame.JOINT_STEREO)
	s.wr.Encoder.SetVBR(lame.VBR_RH)
	s.wr.Encoder.InitParams()

	return func(b [][]float64) error {
		buf := new(bytes.Buffer)
		ints := signal.Float64(b).AsInterInt(signal.BitDepth16)
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

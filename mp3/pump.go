package mp3

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/pipelined/phono"

	mp3 "github.com/hajimehoshi/go-mp3"
)

// Pump allows to read mp3 files.
type Pump struct {
	bufferSize  int
	numChannels int
	f           *os.File
	d           *mp3.Decoder
}

// NewPump creates new mp3 Pump.
// NOTE: current decoder always provides stereo, so return constant.
func NewPump(path string, bufferSize int) (*Pump, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	d, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, err
	}

	return &Pump{
		d:           d,
		f:           f,
		bufferSize:  bufferSize,
		numChannels: 2,
	}, nil
}

// Pump reads buffer from mp3.
func (p *Pump) Pump(string) (func() (phono.Buffer, error), error) {
	return func() (phono.Buffer, error) {
		capacity := p.bufferSize * p.numChannels
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

		b := phono.EmptyBuffer(p.numChannels, len(ints)/p.numChannels)
		b.ReadInts(ints)
		// read not enough samples
		if b.Size() != p.bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, nil
}

// Flush all buffers.
func (p *Pump) Flush(string) error {
	return p.d.Close()
}

// SampleRate returns sample rate of decoded file.
func (p *Pump) SampleRate() int {
	if p == nil || p.d == nil {
		return 0
	}
	return p.d.SampleRate()
}

// NumChannels returns number of channels.
func (p *Pump) NumChannels() int {
	if p == nil {
		return 0
	}
	return p.numChannels
}

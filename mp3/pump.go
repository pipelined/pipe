package mp3

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/pipelined/phono"

	"github.com/hajimehoshi/go-mp3"
)

// Pump allows to read mp3 files.
type Pump struct {
	phono.UID
	f          *os.File
	d          *mp3.Decoder
	bufferSize phono.BufferSize
	done       bool
}

// NewPump creates new mp3 Pump.
func NewPump(path string, bufferSize phono.BufferSize) (*Pump, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	d, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, err
	}

	return &Pump{
		d:          d,
		f:          f,
		UID:        phono.NewUID(),
		bufferSize: bufferSize,
	}, nil
}

// Pump reads buffer from mp3.
func (p *Pump) Pump(string) (phono.PumpFunc, error) {
	return func() (phono.Buffer, error) {
		if p.done {
			return nil, phono.ErrEOP
		}

		ints := make([]int, 0, int(p.bufferSize)*2)
		var val int16
		for len(ints) < int(p.bufferSize)*2 && !p.done {
			if err := binary.Read(p.d, binary.LittleEndian, &val); err != nil {
				if err == io.EOF {
					p.done = true
				} else {
					return nil, err
				}
			}
			ints = append(ints, int(val))
		}
		if len(ints)%2 == 1 {
			ints = append(ints, 0)
		}
		b := phono.EmptyBuffer(2, phono.BufferSize(len(ints)))
		b.ReadInts(ints)
		return b, nil
	}, nil
}

// Flush all buffers.
func (p *Pump) Flush(string) error {
	return p.d.Close()
}

// SampleRate returns sample rate of decoded file.
func (p *Pump) SampleRate() phono.SampleRate {
	if p == nil || p.d == nil {
		return 0
	}
	return phono.SampleRate(p.d.SampleRate())
}

// NumChannels returns number of channels.
// NOTE: current decoder always provides stereo, so return constant.
func (p *Pump) NumChannels() phono.NumChannels {
	return 2
}

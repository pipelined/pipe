package mp3

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pipelined/phono/signal"

	"github.com/pipelined/phono"

	mp3 "github.com/hajimehoshi/go-mp3"
)

// Pump allows to read mp3 files.
type Pump struct {
	path        string
	bufferSize  int
	numChannels int
	sampleRate  int
	f           *os.File
	d           *mp3.Decoder
}

// NewPump creates new mp3 Pump.
func NewPump(path string, bufferSize int) *Pump {
	return &Pump{
		path:       path,
		bufferSize: bufferSize,
	}
}

// Pump reads buffer from mp3.
func (p *Pump) Pump(string) (func() (phono.Buffer, error), int, int, error) {
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
	p.numChannels = 2

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

		b := phono.Buffer(signal.InterInt{Data: ints, NumChannels: p.numChannels, BitDepth: signal.BitDepth16}.AsFloat64())
		// read not enough samples
		if b.Size() != p.bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, p.sampleRate, p.numChannels, nil
}

// Flush all buffers.
func (p *Pump) Flush(string) error {
	return p.d.Close()
}

package vst2

import (
	"log"
	"math"
	"time"
	"unsafe"

	"github.com/pipelined/phono"
	"github.com/pipelined/vst2"
)

// Processor represents vst2 sound processor
type Processor struct {
	plugin *vst2.Plugin

	bufferSize    int
	numChannels   int
	sampleRate    int
	tempo         float32
	timeSignature vst2.TimeSignature

	currentPosition int64
}

// NewProcessor creates new vst2 processor.
func NewProcessor(plugin *vst2.Plugin, bufferSize int, sampleRate int, numChannels int) *Processor {
	return &Processor{
		plugin:          plugin,
		currentPosition: 0,
		bufferSize:      bufferSize,
		sampleRate:      sampleRate,
		numChannels:     numChannels,
	}
}

// Process returns processor function with default settings initialized.
func (p *Processor) Process(string) (func(phono.Buffer) (phono.Buffer, error), error) {
	p.plugin.SetCallback(p.callback())
	p.plugin.SetBufferSize(int(p.bufferSize))
	p.plugin.SetSampleRate(int(p.sampleRate))
	p.plugin.SetSpeakerArrangement(int(p.numChannels))
	p.plugin.Resume()
	return func(b phono.Buffer) (phono.Buffer, error) {
		b = p.plugin.Process(b)
		p.currentPosition += int64(b.Size())
		return b, nil
	}, nil
}

// Flush suspends plugin.
func (p *Processor) Flush(string) error {
	p.plugin.Suspend()
	return nil
}

// wraped callback with session.
func (p *Processor) callback() vst2.HostCallbackFunc {
	return func(plugin *vst2.Plugin, opcode vst2.MasterOpcode, index int64, value int64, ptr unsafe.Pointer, opt float64) int {
		switch opcode {
		case vst2.AudioMasterIdle:
			log.Printf("AudioMasterIdle")
			plugin.Dispatch(vst2.EffEditIdle, 0, 0, nil, 0)

		case vst2.AudioMasterGetCurrentProcessLevel:
			//TODO: return C.kVstProcessLevel
		case vst2.AudioMasterGetSampleRate:
			return int(p.sampleRate)
		case vst2.AudioMasterGetBlockSize:
			return int(p.bufferSize)
		case vst2.AudioMasterGetTime:
			nanoseconds := time.Now().UnixNano()
			notesPerMeasure := p.timeSignature.NotesPerBar
			//TODO: make this dynamic (handle time signature changes)
			// samples position
			samplePos := p.currentPosition
			// todo tempo
			tempo := p.tempo

			samplesPerBeat := (60.0 / float64(tempo)) * float64(p.sampleRate)
			// todo: ppqPos
			ppqPos := float64(samplePos)/samplesPerBeat + 1.0
			// todo: barPos
			barPos := math.Floor(ppqPos / float64(notesPerMeasure))

			return int(plugin.SetTimeInfo(int(p.sampleRate), samplePos, float32(tempo), p.timeSignature, nanoseconds, ppqPos, barPos))
		default:
			// log.Printf("Plugin requested value of opcode %v\n", opcode)
			break
		}
		return 0
	}
}

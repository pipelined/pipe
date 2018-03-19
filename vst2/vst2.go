package vst2

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/dudk/phono"
	"github.com/dudk/vst2"
)

// Processor represents vst2 sound processor
type Processor struct {
	plugin *Plugin

	samplePos uint64
}

// NewProcessor creates new vst2 processor
func NewProcessor(plugin *Plugin) *Processor {
	return &Processor{
		plugin:    plugin,
		samplePos: 0,
	}
}

// Process implements processor.Processor
func (p *Processor) Process(s phono.Session) phono.ProcessFunc {
	p.plugin.SetCallback(callbackWithSession(s))
	return func(ctx context.Context, in <-chan phono.Message) (<-chan phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		out := make(chan phono.Message)
		go func() {
			defer close(out)
			defer close(errc)
			p.plugin.BufferSize(s.BufferSize())
			p.plugin.SampleRate(s.SampleRate())
			p.plugin.SetSpeakerArrangement(2)
			p.plugin.Resume()
			defer p.plugin.Suspend()
			for in != nil {
				select {
				case message, ok := <-in:
					if !ok {
						in = nil
					} else {
						samples := message.Samples()
						processed := p.plugin.Process(samples)
						message.PutSamples(processed)
						out <- message
						atomic.AddUint64(&p.samplePos, uint64(len(samples[0])))
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, errc, nil
	}
}

//Open loads a library
func Open(path string) (*Library, error) {
	lib, err := vst2.Open(path)
	if err != nil {
		return nil, err
	}
	return &Library{
		Library: lib,
	}, nil
}

//Plugin is a wrapper for vst2.Plugin
type Plugin struct {
	*vst2.Plugin
}

//Library is a wrapper over vst2 sdk type
type Library struct {
	*vst2.Library
}

//Open loads vst2 plugin
func (l Library) Open() (p *Plugin, err error) {
	plugin, err := l.Library.Open()
	if err != nil {
		return nil, err
	}
	p = &Plugin{
		Plugin: plugin,
	}
	plugin.SetCallback(p.callback())
	return
}

// DefaultScanPaths returns a slice of default vst2 locations
func DefaultScanPaths() (paths []string) {
	switch goos := runtime.GOOS; goos {
	case "darwin":
		paths = []string{
			"~/Library/Audio/Plug-Ins/VST",
			"/Library/Audio/Plug-Ins/VST",
		}
	case "windows":
		paths = []string{
			"C:\\Program Files (x86)\\Steinberg\\VSTPlugins",
			"C:\\Program Files\\Steinberg\\VSTPlugins ",
		}
		envVstPath := os.Getenv("VST_PATH")
		if len(envVstPath) > 0 {
			paths = append(paths, envVstPath)
		}
	}
	return
}

// FileExtension returns default vst2 extension
func FileExtension() string {
	switch os := runtime.GOOS; os {
	case "darwin":
		return ".vst"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

// Resume starts the plugin
func (p *Plugin) Resume() {
	p.Dispatch(vst2.EffMainsChanged, 0, 1, nil, 0.0)
}

// Suspend stops the plugin
func (p *Plugin) Suspend() {
	p.Dispatch(vst2.EffMainsChanged, 0, 0, nil, 0.0)
}

// BufferSize sets a buffer size
func (p *Plugin) BufferSize(bufferSize int) {
	p.Dispatch(vst2.EffSetBlockSize, 0, int64(bufferSize), nil, 0.0)
}

// SampleRate sets a sample rate for plugin
func (p *Plugin) SampleRate(sampleRate int) {
	p.Dispatch(vst2.EffSetSampleRate, 0, 0, nil, float64(sampleRate))
}

func (p *Plugin) callback() vst2.HostCallbackFunc {
	return func(plugin *vst2.Plugin, opcode vst2.MasterOpcode, index int64, value int64, ptr unsafe.Pointer, opt float64) int {
		fmt.Printf("Printf from closure callback! Plugin name: %v\n", p.Name)
		return 0
	}
}

// Process is a wrapper over ProcessFloat64 and ProcessFloat32
// in case if plugin supports only ProcessFloat32, coversion is done
func (p *Plugin) Process(in [][]float64) [][]float64 {
	if p.Plugin.CanProcessFloat32() {

		in32 := make([][]float32, len(in))
		for i := range in {
			in32[i] = make([]float32, len(in[i]))
			for j, v := range in[i] {
				in32[i][j] = float32(v)
			}
		}

		out32 := p.ProcessFloat32(in32)

		out := make([][]float64, len(out32))
		for i := range out32 {
			out[i] = make([]float64, len(out32[i]))
			for j, v := range out32[i] {
				out[i][j] = float64(v)
			}
		}
		return out
	}
	return p.ProcessFloat64(in)
}

func callbackWithSession(s phono.Session) vst2.HostCallbackFunc {
	return func(p *vst2.Plugin, opcode vst2.MasterOpcode, index int64, value int64, ptr unsafe.Pointer, opt float64) int {
		switch opcode {

		}
		return 0
	}
}

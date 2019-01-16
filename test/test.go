// Package test contains helper functions usefull for testing phono packages.
package test

import (
	"path/filepath"
)

// All test assets should be listed here so they could be accessible in all test packages.
var (
	testdata = "../_testdata/"
	out      = "out/"
	Vst      = resolvePath(testdata + "Krush.vst")

	// All input resources and some attributes.
	Data = struct {
		Wav1        string
		Wav2        string
		Mp3         string
		Wav1Samples int
	}{
		Wav1:        resolvePath(testdata + "sample1.wav"), // Wav1 is the wav file with bass slap sample.
		Wav2:        resolvePath(testdata + "sample2.wav"), // Wav2 is the wav file with trimmed reversed bass slap sample.
		Mp3:         resolvePath(testdata + "sample.mp3"),
		Wav1Samples: 330534,
	}

	// List of all outputs to avoid collision.
	Out = struct {
		Wav1     string
		Wav2     string
		Track    string
		Example2 string
		Example3 string
		Example4 string
		Example5 string
		Mixer    string
		Mp3      string
	}{
		Wav1:     resolvePath(testdata + out + "wav1.wav"),
		Wav2:     resolvePath(testdata + out + "wav2.wav"),
		Track:    resolvePath(testdata + out + "track.wav"),
		Example2: resolvePath(testdata + out + "example2.wav"),
		Example3: resolvePath(testdata + out + "example3.wav"),
		Example4: resolvePath(testdata + out + "example4.wav"),
		Example5: resolvePath(testdata + out + "example5.wav"),
		Mixer:    resolvePath(testdata + out + "mixer.wav"),
		Mp3:      resolvePath(testdata + out + "mp3.mp3"),
	}
)

func resolvePath(path string) string {
	result, _ := filepath.Abs(path)
	return result
}

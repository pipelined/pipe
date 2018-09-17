package example

import (
	"testing"

	"github.com/dudk/phono/mock"
)

func TestOne(t *testing.T) {
	if mock.SkipPortaudio {
		t.Skip("Skip example.TestOne")
	}
	one()
}

func TestTwo(t *testing.T) {
	two()
}

func TestThree(t *testing.T) {
	three()
}

func TestFour(t *testing.T) {
	if mock.SkipPortaudio {
		t.Skip("Skip example.TestFour")
	}
	four()
}

func TestFive(t *testing.T) {
	five()
}

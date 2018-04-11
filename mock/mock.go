package mock

import (
	"github.com/dudk/phono"
)

const (
	defaultBufferSize = 512
	defaultSampleRate = 44100
)

// Option types
type (
	// SimpleOption represents int64-valued option
	SimpleOption int64
	// ComplexOption represents  key-valued struct option
	ComplexOption struct {
		Key   string
		Value interface{}
	}

	// OptionUser is a simple option-user
	OptionUser struct {
		Simple  SimpleOption
		Complex ComplexOption
	}
	// SimpleOptionUser is a simple option-user with multiple values
	SimpleOptionUser struct {
		Simple1 SimpleOption
		Simple2 SimpleOption
	}
)

// Validate implements a phono.OptionUser interface
func (ou *OptionUser) Validate() error {
	return nil
}

// WithSimple assignes a SimpleOption to an OptionUser
func (ou *OptionUser) WithSimple(v SimpleOption) phono.OptionFunc {
	return func() {
		ou.Simple = v
	}
}

// WithComplex assignes a ComplexOption to an OptionUser
func (ou *OptionUser) WithComplex(v ComplexOption) phono.OptionFunc {
	return func() {
		ou.Complex = v
	}
}

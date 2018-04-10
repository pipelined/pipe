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

// Use implements option user interface
func (ou *OptionUser) Use(v phono.OptionValue) {
	switch t := v.(type) {
	case SimpleOption:
		ou.Simple = t
	case ComplexOption:
		ou.Complex = t
	}
}

func (ou *OptionUser) UseSimple(v SimpleOption) {
	ou.Simple = v
}

func (ou *OptionUser) Validate() error {
	return nil
}

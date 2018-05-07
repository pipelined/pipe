package phono_test

import (
	"testing"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleOptions(t *testing.T) {
	ou := &mock.OptionUser{}
	s := ou.WithSimple(mock.SimpleOption(10))
	c := ou.WithComplex(
		mock.ComplexOption{
			Key:   "test",
			Value: 20,
		})
	op := phono.NewOptions().AddOptionsFor(ou, s).AddOptionsFor(ou, c)
	op.ApplyTo(ou)

	assert.Equal(t, mock.SimpleOption(10), ou.Simple)
	assert.Equal(t, "test", ou.Complex.Key)
	assert.Equal(t, 20, ou.Complex.Value)
}

func TestMergeOptions(t *testing.T) {
	var options *phono.Options
	ou := &mock.OptionUser{}

	simple1 := ou.WithSimple(mock.SimpleOption(10))
	newOptions1 := phono.NewOptions().AddOptionsFor(ou, simple1)
	options = options.Join(newOptions1)
	options.ApplyTo(ou)
	assert.Equal(t, mock.SimpleOption(10), ou.Simple)

	simple2 := ou.WithSimple(mock.SimpleOption(20))
	newOptions2 := phono.NewOptions().AddOptionsFor(ou, simple2)
	options = options.Join(newOptions2)
	options.ApplyTo(ou)
	assert.Equal(t, mock.SimpleOption(20), ou.Simple)
}

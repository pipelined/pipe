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
	op := phono.NewOptions().Add(ou, s).Add(ou, c)
	op.ApplyTo(ou)

	assert.Equal(t, mock.SimpleOption(10), ou.Simple)
	assert.Equal(t, "test", ou.Complex.Key)
	assert.Equal(t, 20, ou.Complex.Value)
}

// func TestNewMessage(t *testing.T) {
// 	// ou := &mock.OptionUser{}
// 	// options := phono.NewOptions()
// 	// options.Add(ou, ou.WithSimple(mock.SimpleOption(10)))
// 	// factory := phono.NewMessage()
// 	// message := factory(options)
// 	// assert.NotNil(t, message.Options)
// 	// message = factory(options)
// 	// assert.Nil(t, message.Options)
// }

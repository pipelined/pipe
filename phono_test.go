package phono_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
)

func TestSimpleOptions(t *testing.T) {
	ou := &mock.OptionUser{}
	p := phono.NewOptions().
		Public(mock.SimpleOption(10)).
		Public(mock.ComplexOption{Key: "test", Value: 20})
	p.ApplyTo(ou)
	assert.Equal(t, mock.SimpleOption(10), ou.Simple)
	assert.Equal(t, "test", ou.Complex.Key)
	assert.Equal(t, 20, ou.Complex.Value)
}

func BenchmarkSimpleOption(b *testing.B) {
	ou := &mock.OptionUser{}
	for i := 0; i < b.N; i++ {
		ou.Use(mock.SimpleOption(i))
	}
}

func BenchmarkSimpleValue(b *testing.B) {
	ou := &mock.OptionUser{}
	for i := 0; i < b.N; i++ {
		ou.UseSimple(mock.SimpleOption(i))
	}
}

package phono_test

import (
	"testing"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	ou := &mock.ParamUser{}
	s := ou.WithSimple(mock.SimpleParam(10))
	c := ou.WithComplex(
		mock.ComplexParam{
			Key:   "test",
			Value: 20,
		})
	op := phono.NewParams().Add(ou, s).Add(ou, c)
	op.ApplyTo(ou)

	assert.Equal(t, mock.SimpleParam(10), ou.Simple)
	assert.Equal(t, "test", ou.Complex.Key)
	assert.Equal(t, 20, ou.Complex.Value)
}

func TestMergeParams(t *testing.T) {
	var params *phono.Params
	ou := &mock.ParamUser{}

	simple1 := ou.WithSimple(mock.SimpleParam(10))
	newParams1 := phono.NewParams().Add(ou, simple1)
	params = params.Join(newParams1)
	params.ApplyTo(ou)
	assert.Equal(t, mock.SimpleParam(10), ou.Simple)

	simple2 := ou.WithSimple(mock.SimpleParam(20))
	newParams2 := phono.NewParams().Add(ou, simple2)
	params = params.Join(newParams2)
	params.ApplyTo(ou)
	assert.Equal(t, mock.SimpleParam(20), ou.Simple)
}

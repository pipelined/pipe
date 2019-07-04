package pipe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	expected, value := 10, 0
	fn := func() {
		value = 10
	}
	p := params(make(map[string][]func()))
	uid := newUID()
	p = p.add(uid, fn)
	p.applyTo(uid)

	assert.Equal(t, expected, value)
}

func TestMergeParams(t *testing.T) {
	var p, newP params
	expected, value := 10, 0
	fn := func() {
		value += 10
	}

	p = make(map[string][]func())
	newP = make(map[string][]func())
	uid := newUID()
	newP.add(uid, fn)
	p = p.merge(newP)
	p.applyTo(uid)
	assert.Equal(t, expected, value)

	expected = 20
	newP = make(map[string][]func())
	newP = newP.add(uid, fn)
	p = p.merge(newP)
	p.applyTo(uid)
	assert.Equal(t, expected, value)
}

package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe/internal/state"
)

func TestSimpleParams(t *testing.T) {
	expected, value := 10, 0
	fn := func() {
		value = 10
	}
	p := state.Params(make(map[string][]func()))
	uid := "testID"
	p = p.Add(uid, fn)
	p.ApplyTo(uid)

	assert.Equal(t, expected, value)
}

func TestMergeParams(t *testing.T) {
	var p, newP state.Params
	expected, value := 10, 0
	fn := func() {
		value += 10
	}

	p = make(map[string][]func())
	newP = make(map[string][]func())
	uid := "newUID"
	newP.Add(uid, fn)
	p = p.Merge(newP)
	p.ApplyTo(uid)
	assert.Equal(t, expected, value)

	expected = 20
	newP = make(map[string][]func())
	newP = newP.Add(uid, fn)
	p = p.Merge(newP)
	p.ApplyTo(uid)
	assert.Equal(t, expected, value)
}

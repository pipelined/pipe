package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	//check if default scan paths are presented
	assert.NotEqual(t, len(vstPaths), 0)
}

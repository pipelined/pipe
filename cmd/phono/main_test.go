package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	//check if commands are registered
	assert.Equal(t, len(commands), 1)
}

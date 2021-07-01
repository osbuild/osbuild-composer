package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntToPtr(t *testing.T) {
	var value int = 42
	got := IntToPtr(value)
	assert.Equal(t, value, *got)
}

func TestBoolToPtr(t *testing.T) {
	var value bool = true
	got := BoolToPtr(value)
	assert.Equal(t, value, *got)
}

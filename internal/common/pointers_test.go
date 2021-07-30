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

func TestUint64ToPtr(t *testing.T) {
	var value uint64 = 1
	got := Uint64ToPtr(value)
	assert.Equal(t, value, *got)
}

func TestStringToPtr(t *testing.T) {
	var value string = "the-greatest-test-value"
	got := StringToPtr(value)
	assert.Equal(t, value, *got)
}

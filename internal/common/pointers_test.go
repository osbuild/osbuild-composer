package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPtr(t *testing.T) {
	var valueInt int = 42
	gotInt := ToPtr(valueInt)
	assert.Equal(t, valueInt, *gotInt)

	var valueBool bool = true
	gotBool := ToPtr(valueBool)
	assert.Equal(t, valueBool, *gotBool)

	var valueUint64 uint64 = 1
	gotUint64 := ToPtr(valueUint64)
	assert.Equal(t, valueUint64, *gotUint64)

	var valueStr string = "the-greatest-test-value"
	gotStr := ToPtr(valueStr)
	assert.Equal(t, valueStr, *gotStr)

}

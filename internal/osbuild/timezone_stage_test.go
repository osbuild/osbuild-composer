package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTimezoneStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.timezone",
		Options: &TimezoneStageOptions{},
	}
	actualStage := NewTimezoneStage(&TimezoneStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewZiplStageOptions(t *testing.T) {
	expectedOptions := &ZiplStageOptions{
		Timeout: 0,
	}
	actualOptions := NewZiplStageOptions()
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewZiplStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.zipl",
		Options: &ZiplStageOptions{},
	}
	actualStage := NewZiplStage(&ZiplStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

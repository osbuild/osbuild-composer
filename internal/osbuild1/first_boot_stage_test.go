package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFirstBootStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.first-boot",
		Options: &FirstBootStageOptions{},
	}
	actualStage := NewFirstBootStage(&FirstBootStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

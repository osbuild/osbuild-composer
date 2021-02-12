package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRPMStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.rpm",
		Options: &RPMStageOptions{},
	}
	actualStage := NewRPMStage(&RPMStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

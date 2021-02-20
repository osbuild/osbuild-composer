package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRPMStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rpm",
		Options: &RPMStageOptions{},
		Inputs:  &RPMStageInputs{},
	}
	actualStage := NewRPMStage(&RPMStageOptions{}, &RPMStageInputs{})
	assert.Equal(t, expectedStage, actualStage)
}

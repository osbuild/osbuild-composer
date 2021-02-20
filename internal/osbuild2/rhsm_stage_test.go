package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRhsmStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rhsm",
		Options: &RHSMStageOptions{},
	}
	actualStage := NewRHSMStage(&RHSMStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

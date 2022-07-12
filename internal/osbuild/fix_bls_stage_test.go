package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFixBLSStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.fix-bls",
		Options: &FixBLSStageOptions{},
	}
	actualStage := NewFixBLSStage(&FixBLSStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

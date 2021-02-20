package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFixBLSStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.fix-bls",
		Options: &FixBLSStageOptions{},
	}
	actualStage := NewFixBLSStage()
	assert.Equal(t, expectedStage, actualStage)
}

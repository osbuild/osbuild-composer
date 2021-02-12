package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFixBLSStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.fix-bls",
		Options: &FixBLSStageOptions{},
	}
	actualStage := NewFixBLSStage()
	assert.Equal(t, expectedStage, actualStage)
}

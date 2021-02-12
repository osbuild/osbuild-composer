package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChronyStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.chrony",
		Options: &ChronyStageOptions{},
	}
	actualStage := NewChronyStage(&ChronyStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChronyStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.chrony",
		Options: &ChronyStageOptions{},
	}
	actualStage := NewChronyStage(&ChronyStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

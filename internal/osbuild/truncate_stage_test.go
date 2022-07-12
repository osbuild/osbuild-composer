package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTruncateStage(t *testing.T) {
	options := TruncateStageOptions{
		Filename: "image.raw",
		Size:     "42G",
	}
	expectedStage := &Stage{
		Type:    "org.osbuild.truncate",
		Options: &options,
	}
	actualStage := NewTruncateStage(&options)
	assert.Equal(t, expectedStage, actualStage)
}

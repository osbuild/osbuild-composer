package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAuthselectStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.authselect",
		Options: &AuthselectStageOptions{},
	}
	actualStage := NewAuthselectStage(&AuthselectStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

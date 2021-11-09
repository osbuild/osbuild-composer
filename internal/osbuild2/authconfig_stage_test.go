package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAuthconfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.authconfig",
		Options: &AuthconfigStageOptions{},
	}
	actualStage := NewAuthconfigStage(&AuthconfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

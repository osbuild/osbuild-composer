package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSysconfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.sysconfig",
		Options: &SysconfigStageOptions{},
	}
	actualStage := NewSysconfigStage(&SysconfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

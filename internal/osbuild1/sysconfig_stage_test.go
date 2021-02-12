package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSysconfigStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.sysconfig",
		Options: &SysconfigStageOptions{},
	}
	actualStage := NewSysconfigStage(&SysconfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

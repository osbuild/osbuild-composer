package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSELinuxStageOptions(t *testing.T) {
	expectedOptions := &SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
	actualOptions := NewSELinuxStageOptions("etc/selinux/targeted/contexts/files/file_contexts")
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewSELinuxStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.selinux",
		Options: &SELinuxStageOptions{},
	}
	actualStage := NewSELinuxStage(&SELinuxStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

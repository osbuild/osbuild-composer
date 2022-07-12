package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSELinuxConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.selinux.config",
		Options: &SELinuxConfigStageOptions{},
	}
	actualStage := NewSELinuxConfigStage(&SELinuxConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

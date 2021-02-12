package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemdStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.systemd",
		Options: &SystemdStageOptions{},
	}
	actualStage := NewSystemdStage(&SystemdStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

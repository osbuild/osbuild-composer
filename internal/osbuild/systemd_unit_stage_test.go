package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemdUnitStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.systemd.unit",
		Options: &SystemdUnitStageOptions{},
	}
	actualStage := NewSystemdUnitStage(&SystemdUnitStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

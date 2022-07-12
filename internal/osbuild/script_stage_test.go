package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewScriptStageOptions(t *testing.T) {
	expectedOptions := &ScriptStageOptions{
		Script: "/root/test.sh",
	}
	actualOptions := NewScriptStageOptions("/root/test.sh")
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewScriptStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.script",
		Options: &ScriptStageOptions{},
	}
	actualStage := NewScriptStage(&ScriptStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

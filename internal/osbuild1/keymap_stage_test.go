package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKeymapStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.keymap",
		Options: &KeymapStageOptions{},
	}
	actualStage := NewKeymapStage(&KeymapStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

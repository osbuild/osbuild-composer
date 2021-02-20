package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKeymapStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.keymap",
		Options: &KeymapStageOptions{},
	}
	actualStage := NewKeymapStage(&KeymapStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

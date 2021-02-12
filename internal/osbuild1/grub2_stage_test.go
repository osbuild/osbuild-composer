package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGRUB2Stage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.grub2",
		Options: &GRUB2StageOptions{},
	}
	actualStage := NewGRUB2Stage(&GRUB2StageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

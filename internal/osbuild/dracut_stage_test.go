package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDracutStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.dracut",
		Options: &DracutStageOptions{},
	}
	actualStage := NewDracutStage(&DracutStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

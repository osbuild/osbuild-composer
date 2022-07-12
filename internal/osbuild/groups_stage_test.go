package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGroupsStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.groups",
		Options: &GroupsStageOptions{},
	}
	actualStage := NewGroupsStage(&GroupsStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

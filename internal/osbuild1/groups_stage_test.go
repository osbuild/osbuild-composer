package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGroupsStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.groups",
		Options: &GroupsStageOptions{},
	}
	actualStage := NewGroupsStage(&GroupsStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

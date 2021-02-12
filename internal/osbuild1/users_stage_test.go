package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUsersStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.users",
		Options: &UsersStageOptions{},
	}
	actualStage := NewUsersStage(&UsersStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

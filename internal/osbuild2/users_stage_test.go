package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUsersStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.users",
		Options: &UsersStageOptions{},
	}
	actualStage := NewUsersStage(&UsersStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

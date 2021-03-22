package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResolvConfStageStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.resolv-conf",
		Options: &ResolvConfStageOptions{},
	}
	actualStage := NewResolvConfStage(&ResolvConfStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

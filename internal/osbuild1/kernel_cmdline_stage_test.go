package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKernelCmdlineStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.kernel-cmdline",
		Options: &KernelCmdlineStageOptions{},
	}
	actualStage := NewKernelCmdlineStage(&KernelCmdlineStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

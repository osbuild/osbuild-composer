package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHostnameStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.hostname",
		Options: &HostnameStageOptions{},
	}
	actualStage := NewHostnameStage(&HostnameStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

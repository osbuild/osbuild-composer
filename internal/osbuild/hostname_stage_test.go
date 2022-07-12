package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHostnameStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.hostname",
		Options: &HostnameStageOptions{},
	}
	actualStage := NewHostnameStage(&HostnameStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

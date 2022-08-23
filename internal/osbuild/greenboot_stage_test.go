package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGreenbootConfig(t *testing.T) {
	config := &GreenbootConfig{
		MonitorServices: []string{"sshd", "NetworkManager"},
	}
	expectedConfig := &Stage{
		Type:    "org.osbuild.greenboot",
		Options: config,
	}

	actualConfig := NewGreenbootConfig(config)
	assert.Equal(t, expectedConfig, actualConfig)
}

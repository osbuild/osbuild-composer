package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFirewallStage(t *testing.T) {
	expectedFirewall := &Stage{
		Type:    "org.osbuild.firewall",
		Options: &FirewallStageOptions{},
	}
	actualFirewall := NewFirewallStage(&FirewallStageOptions{})
	assert.Equal(t, expectedFirewall, actualFirewall)
}

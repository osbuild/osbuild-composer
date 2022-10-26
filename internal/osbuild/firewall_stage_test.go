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

func TestFirewallStageZones_ValidateInvalid(t *testing.T) {
	options := FirewallStageOptions{}
	var sources []string
	options.Zones = append(options.Zones, FirewallZone{
		Name:    "",
		Sources: sources,
	})
	assert := assert.New(t)
	assert.Error(options.validate())
}

package subscription

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/blueprint"
)

// The ImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type ImageOptions struct {
	Organization  string `json:"organization"`
	ActivationKey string `json:"activation_key"`
	ServerUrl     string `json:"server_url"`
	BaseUrl       string `json:"base_url"`
	Insights      bool   `json:"insights"`
	Rhc           bool   `json:"rhc"`
}

type RHSMStatus string

const (
	RHSMConfigWithSubscription RHSMStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMStatus = "no-subscription"
)

// Subscription Manager [rhsm] configuration
type SubManRHSMConfig struct {
	ManageRepos *bool
}

// Subscription Manager [rhsmcertd] configuration
type SubManRHSMCertdConfig struct {
	AutoRegistration *bool
}

// Subscription Manager 'rhsm.conf' configuration
type SubManConfig struct {
	Rhsm      SubManRHSMConfig
	Rhsmcertd SubManRHSMCertdConfig
}

type DNFPluginConfig struct {
	Enabled *bool
}

type SubManDNFPluginsConfig struct {
	ProductID           DNFPluginConfig
	SubscriptionManager DNFPluginConfig
}

type RHSMConfig struct {
	DnfPlugins SubManDNFPluginsConfig
	YumPlugins SubManDNFPluginsConfig
	SubMan     SubManConfig
}

// Clone returns a new instance of RHSMConfig with the same values.
// The returned instance is a deep copy of the original.
func (c *RHSMConfig) Clone() *RHSMConfig {
	if c == nil {
		return nil
	}

	clone := &RHSMConfig{}

	if c.DnfPlugins.ProductID.Enabled != nil {
		clone.DnfPlugins.ProductID.Enabled = common.ToPtr(*c.DnfPlugins.ProductID.Enabled)
	}
	if c.DnfPlugins.SubscriptionManager.Enabled != nil {
		clone.DnfPlugins.SubscriptionManager.Enabled = common.ToPtr(*c.DnfPlugins.SubscriptionManager.Enabled)
	}

	if c.YumPlugins.ProductID.Enabled != nil {
		clone.YumPlugins.ProductID.Enabled = common.ToPtr(*c.YumPlugins.ProductID.Enabled)
	}
	if c.YumPlugins.SubscriptionManager.Enabled != nil {
		clone.YumPlugins.SubscriptionManager.Enabled = common.ToPtr(*c.YumPlugins.SubscriptionManager.Enabled)
	}

	if c.SubMan.Rhsm.ManageRepos != nil {
		clone.SubMan.Rhsm.ManageRepos = common.ToPtr(*c.SubMan.Rhsm.ManageRepos)
	}
	if c.SubMan.Rhsmcertd.AutoRegistration != nil {
		clone.SubMan.Rhsmcertd.AutoRegistration = common.ToPtr(*c.SubMan.Rhsmcertd.AutoRegistration)
	}

	return clone
}

// Update returns a new instance of RHSMConfig with updated values
// from the new RHSMConfig. The original RHSMConfig is not modified and
// the new instance does not share any memory with the original.
func (c *RHSMConfig) Update(new *RHSMConfig) *RHSMConfig {
	c = c.Clone()

	if new == nil {
		return c
	}

	if c == nil {
		c = &RHSMConfig{}
	}

	if new.DnfPlugins.ProductID.Enabled != nil {
		c.DnfPlugins.ProductID.Enabled = common.ToPtr(*new.DnfPlugins.ProductID.Enabled)
	}
	if new.DnfPlugins.SubscriptionManager.Enabled != nil {
		c.DnfPlugins.SubscriptionManager.Enabled = common.ToPtr(*new.DnfPlugins.SubscriptionManager.Enabled)
	}

	if new.YumPlugins.ProductID.Enabled != nil {
		c.YumPlugins.ProductID.Enabled = common.ToPtr(*new.YumPlugins.ProductID.Enabled)
	}
	if new.YumPlugins.SubscriptionManager.Enabled != nil {
		c.YumPlugins.SubscriptionManager.Enabled = common.ToPtr(*new.YumPlugins.SubscriptionManager.Enabled)
	}

	if new.SubMan.Rhsm.ManageRepos != nil {
		c.SubMan.Rhsm.ManageRepos = common.ToPtr(*new.SubMan.Rhsm.ManageRepos)
	}
	if new.SubMan.Rhsmcertd.AutoRegistration != nil {
		c.SubMan.Rhsmcertd.AutoRegistration = common.ToPtr(*new.SubMan.Rhsmcertd.AutoRegistration)
	}

	return c
}

// RHSMConfigFromBP creates a RHSMConfig from a blueprint RHSMCustomization
func RHSMConfigFromBP(bpRHSM *blueprint.RHSMCustomization) *RHSMConfig {
	if bpRHSM == nil || bpRHSM.Config == nil {
		return nil
	}

	c := &RHSMConfig{}

	if plugins := bpRHSM.Config.DNFPlugins; plugins != nil {
		if plugins.ProductID != nil && plugins.ProductID.Enabled != nil {
			c.DnfPlugins.ProductID.Enabled = common.ToPtr(*plugins.ProductID.Enabled)
		}
		if plugins.SubscriptionManager != nil && plugins.SubscriptionManager.Enabled != nil {
			c.DnfPlugins.SubscriptionManager.Enabled = common.ToPtr(*plugins.SubscriptionManager.Enabled)
		}
	}

	// NB: YUMPlugins are not exposed to end users as a customization

	if subMan := bpRHSM.Config.SubscriptionManager; subMan != nil {
		if subMan.RHSMConfig != nil && subMan.RHSMConfig.ManageRepos != nil {
			c.SubMan.Rhsm.ManageRepos = common.ToPtr(*subMan.RHSMConfig.ManageRepos)
		}
		if subMan.RHSMCertdConfig != nil && subMan.RHSMCertdConfig.AutoRegistration != nil {
			c.SubMan.Rhsmcertd.AutoRegistration = common.ToPtr(*subMan.RHSMCertdConfig.AutoRegistration)
		}
	}

	return c
}

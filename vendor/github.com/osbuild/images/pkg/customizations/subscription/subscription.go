package subscription

import (
	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
)

// The ImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type ImageOptions struct {
	Organization  string   `json:"organization"`
	ActivationKey string   `json:"activation_key"`
	ServerUrl     string   `json:"server_url"`
	BaseUrl       string   `json:"base_url"`
	Insights      bool     `json:"insights"`
	Rhc           bool     `json:"rhc"`
	Proxy         string   `json:"proxy"`
	TemplateName  string   `json:"template_name"`
	TemplateUUID  string   `json:"template_uuid"`
	PatchURL      string   `json:"patch_url"`
	ContentSets   []string `json:"content_sets"` // List of repo IDs to enable using subscription-manager on first boot
}

type RHSMStatus string

const (
	RHSMConfigWithSubscription RHSMStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMStatus = "no-subscription"
)

// Subscription Manager [rhsm] configuration
type SubManRHSMConfig struct {
	ManageRepos          *bool `yaml:"manage_repos,omitempty"`
	AutoEnableYumPlugins *bool
}

// Subscription Manager [rhsmcertd] configuration
type SubManRHSMCertdConfig struct {
	AutoRegistration *bool `yaml:"auto_registration,omitempty"`
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
	ProductID           DNFPluginConfig `yaml:"product_id,omitempty"`
	SubscriptionManager DNFPluginConfig `yaml:"subscription_manager,omitempty"`
}

type RHSMConfig struct {
	DnfPlugins SubManDNFPluginsConfig `yaml:"dnf_plugin,omitempty"`
	YumPlugins SubManDNFPluginsConfig `yaml:"yum_plugin,omitempty"`
	SubMan     SubManConfig
}

// Clone returns a new instance of RHSMConfig with the same values.
// The returned instance is a deep copy of the original.
func (c *RHSMConfig) Clone() *RHSMConfig {
	if c == nil {
		return nil
	}

	clone := &RHSMConfig{}

	clone.DnfPlugins.ProductID.Enabled = common.ClonePtr(c.DnfPlugins.ProductID.Enabled)
	clone.DnfPlugins.SubscriptionManager.Enabled = common.ClonePtr(c.DnfPlugins.SubscriptionManager.Enabled)

	clone.YumPlugins.ProductID.Enabled = common.ClonePtr(c.YumPlugins.ProductID.Enabled)
	clone.YumPlugins.SubscriptionManager.Enabled = common.ClonePtr(c.YumPlugins.SubscriptionManager.Enabled)

	clone.SubMan.Rhsm.ManageRepos = common.ClonePtr(c.SubMan.Rhsm.ManageRepos)
	clone.SubMan.Rhsm.AutoEnableYumPlugins = common.ClonePtr(c.SubMan.Rhsm.AutoEnableYumPlugins)
	clone.SubMan.Rhsmcertd.AutoRegistration = common.ClonePtr(c.SubMan.Rhsmcertd.AutoRegistration)

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
		c.DnfPlugins.ProductID.Enabled = common.ClonePtr(new.DnfPlugins.ProductID.Enabled)
	}
	if new.DnfPlugins.SubscriptionManager.Enabled != nil {
		c.DnfPlugins.SubscriptionManager.Enabled = common.ClonePtr(new.DnfPlugins.SubscriptionManager.Enabled)
	}

	if new.YumPlugins.ProductID.Enabled != nil {
		c.YumPlugins.ProductID.Enabled = common.ClonePtr(new.YumPlugins.ProductID.Enabled)
	}
	if new.YumPlugins.SubscriptionManager.Enabled != nil {
		c.YumPlugins.SubscriptionManager.Enabled = common.ClonePtr(new.YumPlugins.SubscriptionManager.Enabled)
	}

	if new.SubMan.Rhsm.ManageRepos != nil {
		c.SubMan.Rhsm.ManageRepos = common.ClonePtr(new.SubMan.Rhsm.ManageRepos)
	}
	if new.SubMan.Rhsm.AutoEnableYumPlugins != nil {
		c.SubMan.Rhsm.AutoEnableYumPlugins = common.ClonePtr(new.SubMan.Rhsm.AutoEnableYumPlugins)
	}
	if new.SubMan.Rhsmcertd.AutoRegistration != nil {
		c.SubMan.Rhsmcertd.AutoRegistration = common.ClonePtr(new.SubMan.Rhsmcertd.AutoRegistration)
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
		if plugins.ProductID != nil {
			c.DnfPlugins.ProductID.Enabled = common.ClonePtr(plugins.ProductID.Enabled)
		}
		if plugins.SubscriptionManager != nil {
			c.DnfPlugins.SubscriptionManager.Enabled = common.ClonePtr(plugins.SubscriptionManager.Enabled)
		}
	}

	// NB: YUMPlugins are not exposed to end users as a customization

	if subMan := bpRHSM.Config.SubscriptionManager; subMan != nil {
		if subMan.RHSMConfig != nil {
			c.SubMan.Rhsm.ManageRepos = common.ClonePtr(subMan.RHSMConfig.ManageRepos)
			c.SubMan.Rhsm.AutoEnableYumPlugins = common.ClonePtr(subMan.RHSMConfig.AutoEnableYumPlugins)
		}
		if subMan.RHSMCertdConfig != nil {
			c.SubMan.Rhsmcertd.AutoRegistration = common.ClonePtr(subMan.RHSMCertdConfig.AutoRegistration)
		}
	}

	return c
}

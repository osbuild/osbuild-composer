package blueprint

// Subscription Manager [rhsm] configuration
type SubManRHSMConfig struct {
	ManageRepos          *bool `json:"manage_repos,omitempty" toml:"manage_repos,omitempty"`
	AutoEnableYumPlugins *bool `json:"auto_enable_yum_plugins,omitempty" toml:"auto_enable_yum_plugins,omitempty"`
}

// Subscription Manager [rhsmcertd] configuration
type SubManRHSMCertdConfig struct {
	AutoRegistration *bool `json:"auto_registration,omitempty" toml:"auto_registration,omitempty"`
}

// Subscription Manager 'rhsm.conf' configuration
type SubManConfig struct {
	RHSMConfig      *SubManRHSMConfig      `json:"rhsm,omitempty" toml:"rhsm,omitempty"`
	RHSMCertdConfig *SubManRHSMCertdConfig `json:"rhsmcertd,omitempty" toml:"rhsmcertd,omitempty"`
}

type DNFPluginConfig struct {
	Enabled *bool `json:"enabled,omitempty" toml:"enabled,omitempty"`
}

type SubManDNFPluginsConfig struct {
	ProductID           *DNFPluginConfig `json:"product_id,omitempty" toml:"product_id,omitempty"`
	SubscriptionManager *DNFPluginConfig `json:"subscription_manager,omitempty" toml:"subscription_manager,omitempty"`
}

type RHSMConfig struct {
	DNFPlugins          *SubManDNFPluginsConfig `json:"dnf_plugins,omitempty" toml:"dnf_plugins,omitempty"`
	SubscriptionManager *SubManConfig           `json:"subscription_manager,omitempty" toml:"subscription_manager,omitempty"`
}

type RHSMCustomization struct {
	Config *RHSMConfig `json:"config,omitempty" toml:"config,omitempty"`
}

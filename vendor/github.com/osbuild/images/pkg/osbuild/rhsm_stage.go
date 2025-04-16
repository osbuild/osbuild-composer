package osbuild

import (
	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
)

// RHSMStageOptions describes configuration of the RHSM stage.
//
// The RHSM stage allows configuration of Red Hat Subscription Manager (RHSM)
// related components. Currently it allows only configuration of the enablement
// state of DNF plugins used by the Subscription Manager.
type RHSMStageOptions struct {
	YumPlugins *RHSMStageOptionsDnfPlugins `json:"yum-plugins,omitempty"`
	DnfPlugins *RHSMStageOptionsDnfPlugins `json:"dnf-plugins,omitempty"`
	SubMan     *RHSMStageOptionsSubMan     `json:"subscription-manager,omitempty"`
}

func (RHSMStageOptions) isStageOptions() {}

// NewRHSMStage creates a new RHSM stage
func NewRHSMStage(options *RHSMStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rhsm",
		Options: options,
	}
}

// RHSMStageOptionsDnfPlugins describes configuration of all RHSM DNF plugins
type RHSMStageOptionsDnfPlugins struct {
	ProductID           *RHSMStageOptionsDnfPlugin `json:"product-id,omitempty"`
	SubscriptionManager *RHSMStageOptionsDnfPlugin `json:"subscription-manager,omitempty"`
}

// RHSMStageOptionsDnfPlugin describes configuration of a specific RHSM DNF
// plugin
//
// Only the enablement state of a DNF plugin can be currenlty  set.
type RHSMStageOptionsDnfPlugin struct {
	Enabled bool `json:"enabled"`
}

// Subscription-manager configuration (/etc/rhsm/rhsm.conf)
type RHSMStageOptionsSubMan struct {
	Rhsm      *SubManConfigRHSMSection      `json:"rhsm,omitempty"`
	Rhsmcertd *SubManConfigRHSMCERTDSection `json:"rhsmcertd,omitempty"`
}

// RHSM configuration section of /etc/rhsm/rhsm.conf
type SubManConfigRHSMSection struct {
	// Whether subscription-manager should manage DNF repos file
	ManageRepos *bool `json:"manage_repos,omitempty"`
	// Whether yum/dnf plugins subscription-manager and product-id should be enabled
	// every-time subscription-manager or subscription-manager-gui is executed
	AutoEnableYumPlugins *bool `json:"auto_enable_yum_plugins,omitempty"`
}

// RHSMCERTD configuration section of /etc/rhsm/rhsm.conf
type SubManConfigRHSMCERTDSection struct {
	// Automatic system registration
	AutoRegistration *bool `json:"auto_registration,omitempty"`
}

func NewRHSMStageOptions(config *subscription.RHSMConfig) *RHSMStageOptions {
	if config == nil {
		return nil
	}

	options := &RHSMStageOptions{}

	dnfPlugProductIdEnabled := config.DnfPlugins.ProductID.Enabled
	dnfPlugSubManEnabled := config.DnfPlugins.SubscriptionManager.Enabled
	if dnfPlugProductIdEnabled != nil || dnfPlugSubManEnabled != nil {
		options.DnfPlugins = &RHSMStageOptionsDnfPlugins{}
		if dnfPlugProductIdEnabled != nil {
			options.DnfPlugins.ProductID = &RHSMStageOptionsDnfPlugin{
				Enabled: *dnfPlugProductIdEnabled,
			}
		}
		if dnfPlugSubManEnabled != nil {
			options.DnfPlugins.SubscriptionManager = &RHSMStageOptionsDnfPlugin{
				Enabled: *dnfPlugSubManEnabled,
			}
		}
	}

	yumPlugProductIdEnabled := config.YumPlugins.ProductID.Enabled
	yumPlugSubManEnabled := config.YumPlugins.SubscriptionManager.Enabled
	if yumPlugProductIdEnabled != nil || yumPlugSubManEnabled != nil {
		options.YumPlugins = &RHSMStageOptionsDnfPlugins{}
		if yumPlugProductIdEnabled != nil {
			options.YumPlugins.ProductID = &RHSMStageOptionsDnfPlugin{
				Enabled: *yumPlugProductIdEnabled,
			}
		}
		if yumPlugSubManEnabled != nil {
			options.YumPlugins.SubscriptionManager = &RHSMStageOptionsDnfPlugin{
				Enabled: *yumPlugSubManEnabled,
			}
		}
	}

	subManConfRhsmManageRepos := config.SubMan.Rhsm.ManageRepos
	subManConfRhsmAutoEnableYumPlugins := config.SubMan.Rhsm.AutoEnableYumPlugins
	subManConfRhsmcertdAutoReg := config.SubMan.Rhsmcertd.AutoRegistration

	subManConfValues := []*bool{
		subManConfRhsmManageRepos,
		subManConfRhsmAutoEnableYumPlugins,
		subManConfRhsmcertdAutoReg,
	}
	if slices.ContainsFunc(subManConfValues, func(val *bool) bool { return val != nil }) {
		options.SubMan = &RHSMStageOptionsSubMan{}

		if subManConfRhsmManageRepos != nil || subManConfRhsmAutoEnableYumPlugins != nil {
			options.SubMan.Rhsm = &SubManConfigRHSMSection{}
			if subManConfRhsmManageRepos != nil {
				options.SubMan.Rhsm.ManageRepos = common.ToPtr(*subManConfRhsmManageRepos)
			}
			if subManConfRhsmAutoEnableYumPlugins != nil {
				options.SubMan.Rhsm.AutoEnableYumPlugins = common.ToPtr(*subManConfRhsmAutoEnableYumPlugins)
			}
		}

		if subManConfRhsmcertdAutoReg != nil {
			options.SubMan.Rhsmcertd = &SubManConfigRHSMCERTDSection{
				AutoRegistration: common.ToPtr(*subManConfRhsmcertdAutoReg),
			}
		}
	}

	return options
}

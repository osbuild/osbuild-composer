package osbuild1

// RHSMStageOptions describes configuration of the RHSM stage.
//
// The RHSM stage allows configuration of Red Hat Subscription Manager (RHSM)
// related components. Currently it allows only configuration of the enablement
// state of DNF plugins used by the Subscription Manager.
type RHSMStageOptions struct {
	DnfPlugins *RHSMStageOptionsDnfPlugins `json:"dnf-plugins,omitempty"`
}

func (RHSMStageOptions) isStageOptions() {}

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

// NewRHSMStage creates a new RHSM stage
func NewRHSMStage(options *RHSMStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rhsm",
		Options: options,
	}
}

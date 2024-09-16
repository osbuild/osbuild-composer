package osbuild

type RHSMFactsStageOptions struct {
	Facts RHSMFacts `json:"facts"`
}

type RHSMFacts struct {
	ApiType            string `json:"image-builder.osbuild-composer.api-type"`
	OpenSCAPProfileID  string `json:"image-builder.insights.openscap-profile-id,omitempty"`
	CompliancePolicyID string `json:"image-builder.insights.compliance-policy-id,omitempty"`
}

func (RHSMFactsStageOptions) isStageOptions() {}

// NewRHSMFactsStage creates a new RHSM stage
func NewRHSMFactsStage(options *RHSMFactsStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rhsm.facts",
		Options: options,
	}
}

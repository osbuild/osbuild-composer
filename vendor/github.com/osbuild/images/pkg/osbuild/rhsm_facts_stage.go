package osbuild

type RHSMFactsStageOptions struct {
	Facts RHSMFacts `json:"facts"`
}

type RHSMFacts struct {
	ApiType            string `json:"image-builder.osbuild-composer.api-type"`
	OpenSCAPProfileID  string `json:"image-builder.insights.compliance-profile-id,omitempty"`
	CompliancePolicyID string `json:"image-builder.insights.compliance-policy-id,omitempty"`
	BlueprintID        string `json:"image-builder.blueprint-id,omitempty"`
}

func (RHSMFactsStageOptions) isStageOptions() {}

// NewRHSMFactsStage creates a new RHSM stage
func NewRHSMFactsStage(options *RHSMFactsStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rhsm.facts",
		Options: options,
	}
}

package osbuild2

// Options for the org.osbuild.ostree.pull stage.
type OSTreePullStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`
}

func (OSTreePullStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.pull stage to pull OSTree commits into an existing repo
func NewOSTreePullStage(options *OSTreePullStageOptions, inputs Inputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.pull",
		Inputs:  inputs,
		Options: options,
	}
}

type OSTreePullStageInput struct {
	inputCommon
	References OSTreePullStageReferences `json:"references"`
}

func (OSTreePullStageInput) isStageInput() {}

type OSTreePullStageInputs struct {
	Commits *OSTreePullStageInput `json:"commits"`
}

func (OSTreePullStageInputs) isStageInputs() {}

type OSTreePullStageReferences map[string]OSTreePullStageReference

func (OSTreePullStageReferences) isReferences() {}

type OSTreePullStageReference struct {
	Ref string `json:"ref"`
}

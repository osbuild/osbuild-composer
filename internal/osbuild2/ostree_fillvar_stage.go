package osbuild2

// Options for the org.osbuild.ostree.fillvar stage.
type OSTreeFillvarStageOptions struct {

	Deployment Deployment `json:"deployment"`
}

type Deployment struct {

	OsName string `json:"osname"`

	Ref string `json:"ref"`
}

func (OSTreeFillvarStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeFillvarStage(options *OSTreeFillvarStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.fillvar",
		Options: options,
	}
}

package osbuild2

type OstreeConfigOptions struct {
	Sysroot SysrootOptions `json:"sysroot"`
}

type SysrootOptions struct {
	ReadOnly bool `json:"readonly"`
}

// Options for the org.osbuild.ostree.config stage.
type OSTreeConfigStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`

	Config OstreeConfigOptions `json:"config"`
}

func (OSTreeConfigStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeConfigStage(options *OSTreeConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.config",
		Options: options,
	}
}

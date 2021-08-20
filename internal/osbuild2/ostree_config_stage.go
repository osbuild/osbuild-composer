package osbuild2

// Options for the org.osbuild.ostree.config stage.
type OSTreeConfigStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`

	Config *OSTreeConfig `json:"config,omitempty"`
}

func (OSTreeConfigStageOptions) isStageOptions() {}

type OSTreeConfig struct {
	// Options concerning the sysroot
	Sysroot *SysrootOptions `json:"sysroot,omitempty"`
}

type SysrootOptions struct {
	ReadOnly *bool `json:"readonly,omitempty"`
}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeConfigStage(options *OSTreeConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.config",
		Options: options,
	}
}

package osbuild2

// Options for the org.osbuild.ostree.deploy stage.
type OSTreeDeployStageOptions struct {

	OsName string `json:"osname"`

	Ref string `json:"ref"`

	Mounts []string `json:"mounts"`

	Rootfs Rootfs `json:"rootfs"`

	KernelOpts []string `json:"kernel_opts"`
}

type Rootfs struct {
	Label string `json:"label"`
}

func (OSTreeDeployStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeDeployStage(options *OSTreeDeployStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.deploy",
		Options: options,
	}
}

package osbuild1

// RPMOSTreeStageOptions configures the invocation of the `rpm-ostree`
// process for generating an ostree commit.
type RPMOSTreeStageOptions struct {
	EtcGroupMembers []string `json:"etc_group_members,omitempty"`
}

func (RPMOSTreeStageOptions) isStageOptions() {}

// NewRPMOSTreeStage creates a new rpm-ostree Stage object.
func NewRPMOSTreeStage(options *RPMOSTreeStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rpm-ostree",
		Options: options,
	}
}

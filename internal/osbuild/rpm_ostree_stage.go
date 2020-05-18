package osbuild

// The RPM-OSTree stage describes how to transform the imgae into an OSTree.
type RPMOSTreeStageOptions struct {
	EtcGroupMembers []string `json:"etc_group_members"`
}

func (RPMOSTreeStageOptions) isStageOptions() {}

// NewLocaleStage creates a new Locale Stage object.
func NewRPMOSTreeStage(options *RPMOSTreeStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rpm-ostree",
		Options: options,
	}
}

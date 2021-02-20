package osbuild2

type OSTreePrepTreeStageOptions struct {
	EtcGroupMembers []string `json:"etc_group_members,omitempty"`
}

func (OSTreePrepTreeStageOptions) isStageOptions() {}

// The OSTree PrepTree (org.osbuild.ostree.preptree) stage transforms the
// tree to an ostree layout.
func NewOSTreePrepTreeStage(options *OSTreePrepTreeStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.preptree",
		Options: options,
	}
}

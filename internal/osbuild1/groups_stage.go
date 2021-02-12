package osbuild1

type GroupsStageOptions struct {
	Groups map[string]GroupsStageOptionsGroup `json:"groups"`
}

func (GroupsStageOptions) isStageOptions() {}

type GroupsStageOptionsGroup struct {
	Name string `json:"name"`
	GID  *int   `json:"gid,omitempty"`
}

func NewGroupsStage(options *GroupsStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.groups",
		Options: options,
	}
}

package osbuild2

import "github.com/osbuild/osbuild-composer/internal/blueprint"

type GroupsStageOptions struct {
	Groups map[string]GroupsStageOptionsGroup `json:"groups"`
}

func (GroupsStageOptions) isStageOptions() {}

type GroupsStageOptionsGroup struct {
	// NOTE: the name field does not exist in the osbuild schema and should be
	// removed
	Name string `json:"name,omitempty"`
	GID  *int   `json:"gid,omitempty"`
}

func NewGroupsStage(options *GroupsStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.groups",
		Options: options,
	}
}

func NewGroupsStageOptions(groups []blueprint.GroupCustomization) *GroupsStageOptions {
	options := GroupsStageOptions{
		Groups: map[string]GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		options.Groups[group.Name] = GroupsStageOptionsGroup{
			GID: group.GID,
		}
	}

	return &options
}

package osbuild1

import "github.com/osbuild/osbuild-composer/internal/blueprint"

type GroupsStageOptions struct {
	Groups map[string]GroupsStageOptionsGroup `json:"groups"`
}

func (GroupsStageOptions) isStageOptions() {}

type GroupsStageOptionsGroup struct {
	Name string `json:"name,omitempty"`
	GID  *int   `json:"gid,omitempty"`
}

func NewGroupsStage(options *GroupsStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.groups",
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

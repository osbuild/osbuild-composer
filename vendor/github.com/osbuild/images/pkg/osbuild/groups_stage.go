package osbuild

import (
	"github.com/osbuild/images/pkg/customizations/users"
)

type GroupsStageOptions struct {
	Groups map[string]GroupsStageOptionsGroup `json:"groups"`
	Force  bool                               `json:"force"`
}

func (GroupsStageOptions) isStageOptions() {}

type GroupsStageOptionsGroup struct {
	GID *int `json:"gid,omitempty"`
}

func NewGroupsStage(options *GroupsStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.groups",
		Options: options,
	}
}

func NewGroupsStageOptions(groups []users.Group) *GroupsStageOptions {
	if len(groups) == 0 {
		return nil
	}

	options := GroupsStageOptions{
		Groups: map[string]GroupsStageOptionsGroup{},

		// --force makes possible to run groupadd with a group name or GID that
		// already exists without failing. Useful for letting users define
		// groups in their configs without worrying about the group existing in
		// the base system or being created from a package, which isn't
		// predictable. Enable it unconditionally for all builds
		Force: true,
	}

	for _, group := range groups {
		options.Groups[group.Name] = GroupsStageOptionsGroup{
			GID: group.GID,
		}
	}

	return &options
}

func GenGroupsStage(groups []users.Group) *Stage {
	return NewGroupsStage(NewGroupsStageOptions(groups))
}

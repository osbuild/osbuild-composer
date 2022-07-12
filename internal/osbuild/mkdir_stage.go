package osbuild

import "os"

// Options for the org.osbuild.ostree.config stage.
type MkdirStageOptions struct {
	Paths []Path `json:"paths"`
}

type Path struct {
	Path string `json:"path"`

	Mode os.FileMode `json:"mode,omitempty"`
}

func (MkdirStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewMkdirStage(options *MkdirStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkdir",
		Options: options,
	}
}

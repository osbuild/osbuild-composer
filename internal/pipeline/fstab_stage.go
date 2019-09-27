package pipeline

import "github.com/google/uuid"

type FSTabStageOptions struct {
	FileSystems []*FSTabEntry `json:"filesystems"`
}

func (FSTabStageOptions) isStageOptions() {}

func NewFSTabStage(options *FSTabStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.fstab",
		Options: options,
	}
}

type FSTabEntry struct {
	UUID    uuid.UUID `json:"uuid"`
	VFSType string    `json:"vfs_type"`
	Path    string    `json:"path"`
	Freq    int64     `json:"freq"`
	PassNo  int64     `json:"passno"`
}

func (options *FSTabStageOptions) AddEntry(entry *FSTabEntry) {
	options.FileSystems = append(options.FileSystems, entry)
}

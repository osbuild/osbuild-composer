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
	Path    string    `json:"path,omitempty"`
	Options string    `json:"options,omitempty"`
	Freq    uint64    `json:"freq,omitempty"`
	PassNo  uint64    `json:"passno,omitempty"`
}

func (options *FSTabStageOptions) AddFilesystem(id uuid.UUID, vfsType string, path string, opts string, freq uint64, passNo uint64) {
	options.FileSystems = append(options.FileSystems, &FSTabEntry{
		UUID:    id,
		VFSType: vfsType,
		Path:    path,
		Options: opts,
		Freq:    freq,
		PassNo:  passNo,
	})
}

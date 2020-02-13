package osbuild

// The FSTabStageOptions describe the content of the /etc/fstab file.
//
// The structure of the options follows the format of /etc/fstab, except
// that filesystem must be identified by their UUID and ommitted fields
// are set to their defaults (if possible).
type FSTabStageOptions struct {
	FileSystems []*FSTabEntry `json:"filesystems"`
}

func (FSTabStageOptions) isStageOptions() {}

// NewFSTabStage creates a now FSTabStage object
func NewFSTabStage(options *FSTabStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.fstab",
		Options: options,
	}
}

// An FSTabEntry represents one line in /etc/fstab. With the one exception
// that the the spec field must be represented as an UUID.
type FSTabEntry struct {
	UUID    string `json:"uuid"`
	VFSType string `json:"vfs_type"`
	Path    string `json:"path,omitempty"`
	Options string `json:"options,omitempty"`
	Freq    uint64 `json:"freq,omitempty"`
	PassNo  uint64 `json:"passno,omitempty"`
}

// AddFilesystem adds one entry to and FSTabStageOptions object.
func (options *FSTabStageOptions) AddFilesystem(id string, vfsType string, path string, opts string, freq uint64, passNo uint64) {
	options.FileSystems = append(options.FileSystems, &FSTabEntry{
		UUID:    id,
		VFSType: vfsType,
		Path:    path,
		Options: opts,
		Freq:    freq,
		PassNo:  passNo,
	})
}

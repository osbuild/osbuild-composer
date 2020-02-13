package osbuild

// The SELinuxStageOptions describe how to apply selinux labels.
//
// A file contexts configuration file is sepcified that describes
// the filesystem labels to apply to the image.
type SELinuxStageOptions struct {
	FileContexts string `json:"file_contexts"`
}

func (SELinuxStageOptions) isStageOptions() {}

// NewSELinuxStageOptions creates a new SELinuxStaeOptions object, with
// the mandatory fields set.
func NewSELinuxStageOptions(fileContexts string) *SELinuxStageOptions {
	return &SELinuxStageOptions{
		FileContexts: fileContexts,
	}
}

// NewSELinuxStage creates a new SELinux Stage object.
func NewSELinuxStage(options *SELinuxStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.selinux",
		Options: options,
	}
}

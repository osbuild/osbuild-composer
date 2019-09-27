package pipeline

type SELinuxStageOptions struct {
	FileContexts string `json:"file_contexts"`
}

func (SELinuxStageOptions) isStageOptions() {}

func NewSELinuxStageOptions(fileContexts string) *SELinuxStageOptions {
	return &SELinuxStageOptions{
		FileContexts: fileContexts,
	}
}

func NewSELinuxStage(options *SELinuxStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.selinux",
		Options: options,
	}
}

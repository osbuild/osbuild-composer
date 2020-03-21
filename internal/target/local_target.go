package target

type LocalTargetOptions struct {
	Filename string `json:"filename"`
}

func (LocalTargetOptions) isTargetOptions() {}

func NewLocalTarget(options *LocalTargetOptions) *Target {
	return newTarget("org.osbuild.local", options)
}

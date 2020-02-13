package target

type LocalTargetOptions struct {
}

func (LocalTargetOptions) isTargetOptions() {}

func NewLocalTarget(options *LocalTargetOptions) *Target {
	return newTarget("org.osbuild.local", options)
}

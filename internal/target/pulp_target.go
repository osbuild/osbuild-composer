package target

const TargetNamePulpOSTree TargetName = "org.osbuild.pulp.ostree"

type PulpOSTreeTargetOptions struct {
	// ServerAddress for the pulp instance
	ServerAddress string `json:"server_address"`

	// Repository to import the ostree commit to
	Repository string `json:"repository"`

	// BasePath for distributing the repository (if new)
	BasePath string `json:"basepath,omitempty"`

	Username string `json:"username"`
	Password string `json:"password"`
}

func (PulpOSTreeTargetOptions) isTargetOptions() {}

func NewPulpOSTreeTarget(options *PulpOSTreeTargetOptions) *Target {
	return newTarget(TargetNamePulpOSTree, options)
}

type PulpOSTreeTargetResultOptions struct {
	RepoURL string `json:"repository_url"`
}

func (PulpOSTreeTargetResultOptions) isTargetResultOptions() {}

func NewPulpOSTreeTargetResult(options *PulpOSTreeTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNamePulpOSTree, options)
}

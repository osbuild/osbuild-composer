package pipeline

// The RPMStageOptions describe the operations of the RPM stage.
//
// The RPM stage downloads and installs a given set of packages. Each package
// is specified by a checksum (its PKGID) and a URL where to fetch it from. GPG
// keys can be specified to verify downloaded RPMs.
type RPMStageOptions struct {
	Packages []PackageSource `json:"packages,omitempty"`
	GPGKeys  []string        `json:"gpgkeys,omitempty"`
}

func (RPMStageOptions) isStageOptions() {}

type PackageSource struct {
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
}

// NewRPMStageOptions creates a new RPMStageOptions object. It contains its
// mandatory fields, but no repositories.
func NewRPMStageOptions() *RPMStageOptions {
	return &RPMStageOptions{
		GPGKeys:  make([]string, 0),
		Packages: make([]PackageSource, 0),
	}
}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rpm",
		Options: options,
	}
}

// AddPackage adds a package to a RPMStageOptions object.
func (options *RPMStageOptions) AddPackage(url, checksum string) {
	options.Packages = append(options.Packages, PackageSource{url, checksum})
}

// AddKey adds a gpg ket to a RPMStageOptions object.
func (options *RPMStageOptions) AddKey(key string) {
	options.GPGKeys = append(options.GPGKeys, key)
}

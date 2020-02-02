package osbuild

// The DNFStageOptions describe the operations of the DNF stage.
//
// The DNF stage installs a given set of packages from a given repository,
// as it was at a given point in time. This is meant to ensure that given
// a set of DNF stage options, the output should be reproducible. If the
// metadata of the repository has changed since the stage options were
// first generated, the stage may fail.
type DNFStageOptions struct {
	Repositories     []*DNFRepository `json:"repos"`
	Packages         []string         `json:"packages"`
	ExcludedPackages []string         `json:"exclude_packages,omitempty"`
	ReleaseVersion   string           `json:"releasever"`
	BaseArchitecture string           `json:"basearch"`
	ModulePlatformId string           `json:"module_platform_id,omitempty"`
}

func (DNFStageOptions) isStageOptions() {}

// A DNFRepository describes one repository at a given point in time, as well
// as the GPG key needed to verify its correctness.
type DNFRepository struct {
	MetaLink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
	BaseURL    string `json:"baseurl,omitempty"`
	GPGKey     string `json:"gpgkey,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

// NewDNFStageOptions creates a new DNFStageOptions object. It contains its
// mandatory fields, but no repositories.
func NewDNFStageOptions(releaseVersion string, baseArchitecture string) *DNFStageOptions {
	return &DNFStageOptions{
		Repositories:     make([]*DNFRepository, 0),
		Packages:         make([]string, 0),
		ReleaseVersion:   releaseVersion,
		BaseArchitecture: baseArchitecture,
	}
}

// NewDNFStage creates a new DNF stage.
func NewDNFStage(options *DNFStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.dnf",
		Options: options,
	}
}

// AddPackage adds a package to a DNFStageOptions object.
func (options *DNFStageOptions) AddPackage(pkg string) {
	options.Packages = append(options.Packages, pkg)
}

// ExcludePackage adds an excluded package to a DNFStageOptions object.
func (options *DNFStageOptions) ExcludePackage(pkg string) {
	options.ExcludedPackages = append(options.ExcludedPackages, pkg)
}

// AddRepository adds a repository to a DNFStageOptions object.
func (options *DNFStageOptions) AddRepository(repo *DNFRepository) {
	options.Repositories = append(options.Repositories, repo)
}

// NewDNFRepository creates a new DNFRepository object. Exactly one of the
// argumnets should not be nil.
func NewDNFRepository(metaLink string, mirrorList string, baseURL string) *DNFRepository {
	// TODO: verify that exactly one argument is non-nil
	return &DNFRepository{
		MetaLink:   metaLink,
		MirrorList: mirrorList,
		BaseURL:    baseURL,
	}
}

// SetGPGKey sets the GPG key for a repository. This is used to verify the
// packages we install.
func (r *DNFRepository) SetGPGKey(gpgKey string) {
	r.GPGKey = gpgKey
}

// SetChecksum sets the metadata checksum of a repository. This is used to
// verify that we only operate on a given version of the repository.
func (r *DNFRepository) SetChecksum(checksum string) {
	r.Checksum = checksum
}

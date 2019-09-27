package pipeline

type DNFStageOptions struct {
	Repositories     map[string]*DNFRepository `json:"repos"`
	Packages         []string                  `json:"packages"`
	ReleaseVersion   string                    `json:"releasever"`
	BaseArchitecture string                    `json:"basearch"`
}

func (DNFStageOptions) isStageOptions() {}

type DNFRepository struct {
	MetaLink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
	BaseURL    string `json:"baseurl,omitempty"`
	GPGKey     string `json:"gpgkey,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

func NewDNFStageOptions(releaseVersion string, baseArchitecture string) *DNFStageOptions {
	return &DNFStageOptions{
		Repositories:     make(map[string]*DNFRepository),
		ReleaseVersion:   releaseVersion,
		BaseArchitecture: baseArchitecture,
	}
}

func NewDNFStage(options *DNFStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.dnf",
		Options: options,
	}
}

func (options *DNFStageOptions) AddPackage(pkg string) {
	options.Packages = append(options.Packages, pkg)
}

func (options *DNFStageOptions) AddRepository(name string, repo *DNFRepository) {
	options.Repositories[name] = repo
}

func NewDNFRepository(metaLink string, mirrorList string, baseURL string) *DNFRepository {
	return &DNFRepository{
		MetaLink:   metaLink,
		MirrorList: mirrorList,
		BaseURL:    baseURL,
	}
}

func (r *DNFRepository) SetGPGKey(gpgKey string) {
	r.GPGKey = gpgKey
}

func (r *DNFRepository) SetChecksum(checksum string) {
	r.Checksum = checksum
}

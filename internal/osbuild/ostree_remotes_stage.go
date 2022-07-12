package osbuild

// Configure OSTree remotes for a repository

// Options for the org.osbuild.ostree.remotes stage.
type OSTreeRemotesStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`

	// Configure remotes for the system repository
	Remotes []OSTreeRemote `json:"remotes,omitempty"`
}

func (OSTreeRemotesStageOptions) isStageOptions() {}

type OSTreeRemote struct {
	// Identifier for the remote
	Name string `json:"name"`

	// URL of the repository.
	URL string `json:"url"`

	// Configured branches for the remote
	Branches []string `json:"branches,omitempty"`

	// GPG keys to verify the commits
	GPGKeys []string `json:"secrets,omitempty"`
}

// A new org.osbuild.ostree.remotes stage to configure remotes
func NewOSTreeRemotesStage(options *OSTreeRemotesStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.remotes",
		Options: options,
	}
}

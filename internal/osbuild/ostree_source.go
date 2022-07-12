package osbuild

// The commits to fetch indexed their checksum
type OSTreeSource struct {
	Items map[string]OSTreeSourceItem `json:"items"`
}

func (OSTreeSource) isSource() {}

type OSTreeSourceItem struct {
	Remote OSTreeSourceRemote `json:"remote"`
}

type OSTreeSourceRemote struct {
	// URL of the repository.
	URL string `json:"url"`
	// GPG keys to verify the commits
	GPGKeys []string `json:"secrets,omitempty"`
}

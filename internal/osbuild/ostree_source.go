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
	URL        string `json:"url"`
	ContentURL string `json:"contenturl,omitempty"`
	// GPG keys to verify the commits
	GPGKeys []string                   `json:"gpgkeys,omitempty"`
	Secrets *OSTreeSourceRemoteSecrets `json:"secrets,omitempty"`
}

type OSTreeSourceRemoteSecrets struct {
	Name string `json:"name"`
}

package blueprint

import (
	"fmt"
)

type RepositoryCustomization struct {
	Id           string   `json:"id" toml:"id"`
	BaseURLs     []string `json:"baseurls,omitempty" toml:"baseurls,omitempty"`
	GPGKeys      []string `json:"gpgkeys,omitempty" toml:"gpgkeys,omitempty"`
	Metalink     string   `json:"metalink,omitempty" toml:"metalink,omitempty"`
	Mirrorlist   string   `json:"mirrorlist,omitempty" toml:"mirrorlist,omitempty"`
	Name         string   `json:"name,omitempty" toml:"name,omitempty"`
	Priority     *int     `json:"priority,omitempty" toml:"priority,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty" toml:"enabled,omitempty"`
	GPGCheck     *bool    `json:"gpgcheck,omitempty" toml:"gpgcheck,omitempty"`
	RepoGPGCheck *bool    `json:"repo_gpgcheck,omitempty" toml:"repo_gpgcheck,omitempty"`
	SSLVerify    bool     `json:"sslverify,omitempty" toml:"sslverify,omitempty"`
	Filename     string   `json:"filename,omitempty" toml:"filename,omitempty"`
}

func validateCustomRepository(repo *RepositoryCustomization) error {
	if repo.Id == "" {
		return fmt.Errorf("Repository ID is required")
	}

	if len(repo.BaseURLs) == 0 && repo.Mirrorlist == "" && repo.Metalink == "" {
		return fmt.Errorf("Repository base URL, mirrorlist or metalink is required")
	}

	if repo.GPGCheck != nil && *repo.GPGCheck && len(repo.GPGKeys) == 0 {
		return fmt.Errorf("Repository gpg check is set to true but no gpg keys are provided")
	}
	return nil
}

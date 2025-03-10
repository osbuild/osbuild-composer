package blueprint

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type RepositoryCustomization struct {
	Id             string   `json:"id" toml:"id"`
	BaseURLs       []string `json:"baseurls,omitempty" toml:"baseurls,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty" toml:"gpgkeys,omitempty"`
	Metalink       string   `json:"metalink,omitempty" toml:"metalink,omitempty"`
	Mirrorlist     string   `json:"mirrorlist,omitempty" toml:"mirrorlist,omitempty"`
	Name           string   `json:"name,omitempty" toml:"name,omitempty"`
	Priority       *int     `json:"priority,omitempty" toml:"priority,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty" toml:"enabled,omitempty"`
	GPGCheck       *bool    `json:"gpgcheck,omitempty" toml:"gpgcheck,omitempty"`
	RepoGPGCheck   *bool    `json:"repo_gpgcheck,omitempty" toml:"repo_gpgcheck,omitempty"`
	SSLVerify      *bool    `json:"sslverify,omitempty" toml:"sslverify,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty" toml:"module_hotfixes,omitempty"`
	Filename       string   `json:"filename,omitempty" toml:"filename,omitempty"`

	// When set the repository will be used during the depsolve of
	// payload repositories to install packages from it.
	InstallFrom bool `json:"install_from" toml:"install_from"`
}

const repoFilenameRegex = "^[\\w.-]{1,250}\\.repo$"

func validateCustomRepository(repo *RepositoryCustomization) error {
	if repo.Id == "" {
		return fmt.Errorf("Repository ID is required")
	}

	filenameRegex := regexp.MustCompile(repoFilenameRegex)
	if !filenameRegex.MatchString(repo.getFilename()) {
		return fmt.Errorf("Repository filename %q is invalid", repo.getFilename())
	}

	if len(repo.BaseURLs) == 0 && repo.Mirrorlist == "" && repo.Metalink == "" {
		return fmt.Errorf("Repository base URL, mirrorlist or metalink is required")
	}

	if repo.GPGCheck != nil && *repo.GPGCheck && len(repo.GPGKeys) == 0 {
		return fmt.Errorf("Repository gpg check is set to true but no gpg keys are provided")
	}

	for _, key := range repo.GPGKeys {
		// check for a valid GPG key prefix & contains GPG suffix
		keyIsGPGKey := strings.HasPrefix(key, "-----BEGIN PGP PUBLIC KEY BLOCK-----") && strings.Contains(key, "-----END PGP PUBLIC KEY BLOCK-----")

		// check for a valid URL
		keyIsURL := false
		_, err := url.ParseRequestURI(key)
		if err == nil {
			keyIsURL = true
		}

		if !keyIsGPGKey && !keyIsURL {
			return fmt.Errorf("Repository gpg key is not a valid URL or a valid gpg key")
		}
	}

	return nil
}

func (rc *RepositoryCustomization) getFilename() string {
	if rc.Filename == "" {
		return fmt.Sprintf("%s.repo", rc.Id)
	}
	if !strings.HasSuffix(rc.Filename, ".repo") {
		return fmt.Sprintf("%s.repo", rc.Filename)
	}
	return rc.Filename
}

package rpmmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type repository struct {
	Name           string   `json:"name"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	IgnoreSSL      bool     `json:"ignore_ssl,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
	PackageSets    []string `json:"package_sets,omitempty"`
}

type RepoConfig struct {
	// the repo id is not always required and is ignored in some cases.
	// For example, it is not required in osbuild-depsolve-dnf, but it is a required
	// field for creating a repo file in `/etc/yum.repos.d/`
	Id             string   `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	BaseURLs       []string `json:"baseurls,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       *bool    `json:"check_gpg,omitempty"`
	CheckRepoGPG   *bool    `json:"check_repo_gpg,omitempty"`
	Priority       *int     `json:"priority,omitempty"`
	IgnoreSSL      *bool    `json:"ignore_ssl,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
	PackageSets    []string `json:"package_sets,omitempty"`

	// These fields are only filled out by the worker during the
	// depsolve job for certain baseurls.
	SSLCACert     string `json:"sslcacert,omitempty"`
	SSLClientKey  string `json:"sslclientkey,omitempty"`
	SSLClientCert string `json:"sslclientcert,omitempty"`
}

// Hash calculates an ID string that uniquely represents a repository
// configuration.  The Name and ImageTypeTags fields are not considered in the
// calculation.
func (r *RepoConfig) Hash() string {
	bts := func(b bool) string {
		return fmt.Sprintf("%T", b)
	}
	bpts := func(b *bool) string {
		if b == nil {
			return ""
		}
		return fmt.Sprintf("%T", b)
	}
	ats := func(s []string) string {
		return strings.Join(s, "")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(ats(r.BaseURLs)+
		r.Metalink+
		r.MirrorList+
		ats(r.GPGKeys)+
		bpts(r.CheckGPG)+
		bpts(r.CheckRepoGPG)+
		bpts(r.IgnoreSSL)+
		r.MetadataExpire+
		bts(r.RHSM)+
		bpts(r.ModuleHotfixes)+
		r.SSLCACert+
		r.SSLClientKey+
		r.SSLClientCert)))
}

type DistrosRepoConfigs map[string]map[string][]RepoConfig

func LoadRepositoriesFromFile(filename string) (map[string][]RepoConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return LoadRepositoriesFromReader(f)
}

func LoadRepositoriesFromReader(r io.Reader) (map[string][]RepoConfig, error) {
	var reposMap map[string][]repository
	repoConfigs := make(map[string][]RepoConfig)

	err := json.NewDecoder(r).Decode(&reposMap)
	if err != nil {
		return nil, err
	}

	for arch, repos := range reposMap {
		for idx := range repos {
			repo := repos[idx]
			var urls []string
			if repo.BaseURL != "" {
				urls = []string{repo.BaseURL}
			}
			var keys []string
			if repo.GPGKey != "" {
				keys = []string{repo.GPGKey}
			}
			if len(repo.GPGKeys) > 0 {
				keys = append(keys, repo.GPGKeys...)
			}
			config := RepoConfig{
				Name:           repo.Name,
				BaseURLs:       urls,
				Metalink:       repo.Metalink,
				MirrorList:     repo.MirrorList,
				GPGKeys:        keys,
				CheckGPG:       &repo.CheckGPG,
				RHSM:           repo.RHSM,
				MetadataExpire: repo.MetadataExpire,
				ModuleHotfixes: repo.ModuleHotfixes,
				ImageTypeTags:  repo.ImageTypeTags,
				PackageSets:    repo.PackageSets,
			}

			repoConfigs[arch] = append(repoConfigs[arch], config)
		}
	}

	return repoConfigs, nil
}

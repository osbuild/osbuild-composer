package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/rpmmd"
)

const SourceNameLibrepo = "org.osbuild.librepo"

// LibrepoSource wraps the org.osbuild.librepo osbuild source
type LibrepoSource struct {
	Items   map[string]*LibrepoSourceItem `json:"items"`
	Options *LibrepoSourceOptions         `json:"options"`
}

func NewLibrepoSource() *LibrepoSource {
	return &LibrepoSource{
		Items: make(map[string]*LibrepoSourceItem),
		Options: &LibrepoSourceOptions{
			Mirrors: make(map[string]*LibrepoSourceMirror),
		},
	}
}

// AddPackage adds the given *depsolved* pkg to the downloading. It
// needs the *depsovled* repoConfig so that the repoID of the two can
// be matched up
func (source *LibrepoSource) AddPackage(pkg rpmmd.Package, repos []rpmmd.RepoConfig) error {
	pkgRepo, err := findRepoById(repos, pkg.RepoID)
	if err != nil {
		return fmt.Errorf("cannot find repo-id for pkg %v: %v", pkg.Name, err)
	}
	if _, ok := source.Options.Mirrors[pkgRepo.Id]; !ok {
		mirror, err := mirrorFromRepo(pkgRepo)
		if err != nil {
			return err
		}
		source.Options.Mirrors[pkgRepo.Id] = mirror
	}
	mirror := source.Options.Mirrors[pkgRepo.Id]
	if pkg.IgnoreSSL {
		mirror.Insecure = true
	}
	// this should never happen but we should still check to avoid
	// potential security issues
	if mirror.Insecure && !pkg.IgnoreSSL {
		return fmt.Errorf("inconsistent SSL configuration: package %v requires SSL but mirror %v is configured to ignore SSL", pkg.Name, mirror.URL)
	}
	switch pkg.Secrets {
	case "org.osbuild.rhsm":
		mirror.Secrets = &URLSecrets{
			Name: "org.osbuild.rhsm",
		}
	case "org.osbuild.mtls":
		mirror.Secrets = &URLSecrets{
			Name: "org.osbuild.mtls",
		}
	}

	item := &LibrepoSourceItem{
		Path:     pkg.Location,
		MirrorID: pkgRepo.Id,
	}
	source.Items[pkg.Checksum.String()] = item
	return nil
}

func (LibrepoSource) isSource() {}

type LibrepoSourceItem struct {
	Path     string `json:"path"`
	MirrorID string `json:"mirror"`
}

func findRepoById(repos []rpmmd.RepoConfig, repoID string) (*rpmmd.RepoConfig, error) {
	type info struct {
		ID   string
		Name string
	}
	var repoInfo []info
	for _, repo := range repos {
		repoInfo = append(repoInfo, info{repo.Id, repo.Name})
		if repo.Id == repoID {
			return &repo, nil
		}
	}

	return nil, fmt.Errorf("cannot find repo-id %v in %+v", repoID, repoInfo)
}

func mirrorFromRepo(repo *rpmmd.RepoConfig) (*LibrepoSourceMirror, error) {
	switch {
	case repo.Metalink != "":
		return &LibrepoSourceMirror{
			URL:  repo.Metalink,
			Type: "metalink",
		}, nil
	case repo.MirrorList != "":
		return &LibrepoSourceMirror{
			URL:  repo.MirrorList,
			Type: "mirrorlist",
		}, nil
	case len(repo.BaseURLs) > 0:
		return &LibrepoSourceMirror{
			// XXX: should we pick a random one instead?
			URL:  repo.BaseURLs[0],
			Type: "baseurl",
		}, nil
	}

	return nil, fmt.Errorf("cannot find metalink, mirrorlist or baseurl for %+v", repo)
}

// librepoSourceOptions are the JSON options for source org.osbuild.librepo
type LibrepoSourceOptions struct {
	Mirrors map[string]*LibrepoSourceMirror `json:"mirrors"`
}

type LibrepoSourceMirror struct {
	URL  string `json:"url"`
	Type string `json:"type"`

	Insecure bool        `json:"insecure,omitempty"`
	Secrets  *URLSecrets `json:"secrets,omitempty"`

	MaxParallels  *int `json:"max-parallels,omitempty"`
	FastestMirror bool `json:"fastest-mirror,omitempty"`
}

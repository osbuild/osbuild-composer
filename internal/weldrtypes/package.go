package weldrtypes

import (
	"fmt"

	"github.com/osbuild/images/pkg/rpmmd"
)

// DepsolvedPackageInfo is the API representation of a package that has been depsolved.
type DepsolvedPackageInfo struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`

	// NB: the fields below are most probably useless for the purpose of this structure.
	// Most of them are omitted when not set or set to default values.
	// Path is useless without the base URL.
	// RepoID is useless because it is a hash of multiple values of the repository used
	// for depsolving, thus it is not useful to the end user.
	Secrets   string `json:"secrets,omitempty"`
	CheckGPG  bool   `json:"check_gpg,omitempty"`
	IgnoreSSL bool   `json:"ignore_ssl,omitempty"`
	Path      string `json:"path,omitempty"`
	RepoID    string `json:"repo_id,omitempty"`
}

func (d DepsolvedPackageInfo) EVRA() string {
	if d.Epoch == 0 {
		return fmt.Sprintf("%s-%s.%s", d.Version, d.Release, d.Arch)
	}
	return fmt.Sprintf("%d:%s-%s.%s", d.Epoch, d.Version, d.Release, d.Arch)
}

func (d DepsolvedPackageInfo) NEVRA() string {
	return fmt.Sprintf("%s-%s", d.Name, d.EVRA())
}

func RPMMDPackageToDepsolvedPackageInfo(pkg rpmmd.Package) DepsolvedPackageInfo {
	p := DepsolvedPackageInfo{
		Name:      pkg.Name,
		Epoch:     pkg.Epoch,
		Version:   pkg.Version,
		Release:   pkg.Release,
		Arch:      pkg.Arch,
		Checksum:  pkg.Checksum.String(),
		Secrets:   pkg.Secrets,
		CheckGPG:  pkg.CheckGPG,
		IgnoreSSL: pkg.IgnoreSSL,
		Path:      pkg.Location,
		RepoID:    pkg.RepoID,
	}
	if len(pkg.RemoteLocations) > 0 {
		p.RemoteLocation = pkg.RemoteLocations[0]
	}
	return p
}

func RPMMDPackageListToDepsolvedPackageInfoList(pkgs rpmmd.PackageList) []DepsolvedPackageInfo {
	results := make([]DepsolvedPackageInfo, len(pkgs))
	for i, pkg := range pkgs {
		results[i] = RPMMDPackageToDepsolvedPackageInfo(pkg)
	}
	return results
}

package v2

import (
	"log"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

func (request *DepsolveRequest) DepsolveBlueprint(df *distrofactory.Factory, rr *reporegistry.RepoRegistry, solver *dnfjson.BaseSolver) ([]rpmmd.PackageSpec, error) {
	bp, err := ConvertRequestBP(request.Blueprint)
	if err != nil {
		return nil, err
	}

	// Distro name, in order of priority
	// bp.Distro
	// host distro
	var originalDistroName string
	if len(bp.Distro) > 0 {
		originalDistroName = bp.Distro
	} else {
		originalDistroName, err = distro.GetHostDistroName()
		if err != nil {
			return nil, HTTPErrorWithInternal(ErrorUnsupportedDistribution, err)
		}
	}

	distribution := df.GetDistro(originalDistroName)
	if distribution == nil {
		return nil, HTTPError(ErrorUnsupportedDistribution)
	}

	var originalArchName string
	if len(bp.Arch) > 0 {
		originalArchName = bp.Arch
	} else {
		originalArchName = arch.Current().String()
	}
	distroArch, err := distribution.GetArch(originalArchName)
	if err != nil {
		return nil, HTTPErrorWithInternal(ErrorUnsupportedArchitecture, err)
	}

	// Get the repositories to use for depsolving
	// Either the list passed in with the request, or the defaults for the distro+arch
	var repos []rpmmd.RepoConfig
	if request.Repositories != nil {
		repos, err = convertRepos(*request.Repositories, []Repository{}, []string{})
		if err != nil {
			// Error comes from genRepoConfig and is already an HTTPError
			return nil, err
		}
	} else {
		repos, err = rr.ReposByArchName(originalDistroName, distroArch.Name(), false)
		if err != nil {
			return nil, HTTPErrorWithInternal(ErrorInvalidRepository, err)
		}
	}

	s := solver.NewWithConfig(
		distribution.ModulePlatformID(),
		distribution.Releasever(),
		distroArch.Name(),
		distribution.Name())
	solved, err := s.Depsolve([]rpmmd.PackageSet{{Include: bp.GetPackages(), Repositories: repos}}, sbom.StandardTypeNone)
	if err != nil {
		return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, err)
	}

	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}

	return solved.Packages, nil
}

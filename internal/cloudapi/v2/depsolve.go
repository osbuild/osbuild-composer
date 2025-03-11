package v2

// DepsolveRequest methods

import (
	"context"
	"fmt"
	"time"

	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func (request *DepsolveRequest) Depsolve(df *distrofactory.Factory, rr *reporegistry.RepoRegistry, workers *worker.Server) ([]rpmmd.PackageSpec, error) {
	// Convert the requested blueprint to a composer blueprint
	bp, err := ConvertRequestBP(request.Blueprint)
	if err != nil {
		return nil, err
	}

	// If the blueprint include distro and/or architecture they must match the ones
	// in request -- otherwise the results may not be what is expected.
	if len(bp.Distro) > 0 && bp.Distro != request.Distribution {
		return nil, HTTPError(ErrorMismatchedDistribution)
	}

	// XXX CloudAPI Blueprint needs to have missing Architecture added first
	/*
		if len(bp.Architecture) > 0 && bp.Architecture != request.Architecture {
			return nil, HTTPError(ErrorMismatchedArchitecture)
		}
	*/
	distro := df.GetDistro(request.Distribution)
	if distro == nil {
		return nil, HTTPError(ErrorUnsupportedDistribution)
	}
	distroArch, err := distro.GetArch(request.Architecture)
	if err != nil {
		return nil, HTTPErrorWithInternal(ErrorUnsupportedArchitecture, err)
	}

	var repos []rpmmd.RepoConfig
	if request.Repositories != nil {
		repos, err = convertRepos(*request.Repositories, []Repository{}, []string{})
		if err != nil {
			// Error comes from genRepoConfig and is already an HTTPError
			return nil, err
		}
	} else {
		repos, err = rr.ReposByArchName(request.Distribution, distroArch.Name(), false)
		if err != nil {
			return nil, HTTPErrorWithInternal(ErrorInvalidRepository, err)
		}
	}

	// Send the depsolve request to the worker
	packageSet := make(map[string][]rpmmd.PackageSet, 1)
	packageSet["depsolve"] = []rpmmd.PackageSet{
		{
			Include:        bp.GetPackages(),
			EnabledModules: bp.GetEnabledModules(),
			Repositories:   repos,
		},
	}

	depsolveJobID, err := workers.EnqueueDepsolve(&worker.DepsolveJob{
		PackageSets:      packageSet,
		ModulePlatformID: distro.ModulePlatformID(),
		Arch:             distroArch.Name(),
		Releasever:       distro.Releasever(),
		SbomType:         sbom.StandardTypeNone,
	}, "")
	if err != nil {
		return nil, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	// Limit how long a depsolve can take
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*depsolveTimeoutMin)
	defer cancel()

	// Wait until depsolve job is finished, fails, or is canceled
	var result worker.DepsolveJobResult
	for {
		time.Sleep(time.Millisecond * 50)
		info, err := workers.DepsolveJobInfo(depsolveJobID, &result)
		if err != nil {
			return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, err)
		}
		if result.JobError != nil {
			return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, err)
		}
		if info.JobStatus != nil {
			if info.JobStatus.Canceled {
				return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, err)
			}

			if !info.JobStatus.Finished.IsZero() {
				break
			}
		}

		select {
		case <-ctx.Done():
			return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, fmt.Errorf("Depsolve job %q timed out", depsolveJobID))
		default:
		}
	}

	return result.PackageSpecs["depsolve"], nil
}

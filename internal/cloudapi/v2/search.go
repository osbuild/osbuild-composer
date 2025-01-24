package v2

// SearchPackagesRequest methods

import (
	"context"
	"fmt"
	"time"

	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func (request *SearchPackagesRequest) Search(df *distrofactory.Factory, rr *reporegistry.RepoRegistry, workers *worker.Server) (rpmmd.PackageList, error) {
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

	// Send the search request to the worker
	searchJobID, err := workers.EnqueueSearchPackages(&worker.SearchPackagesJob{
		Packages:         request.Packages,
		Repositories:     repos,
		ModulePlatformID: distro.ModulePlatformID(),
		Arch:             distroArch.Name(),
		Releasever:       distro.Releasever(),
	}, "")
	if err != nil {
		return nil, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	// Limit how long a search can take
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*depsolveTimeoutMin)
	defer cancel()

	// Wait until search job is finished, fails, or is canceled
	var result worker.SearchPackagesJobResult
	for {
		time.Sleep(time.Millisecond * 50)
		info, err := workers.SearchPackagesJobInfo(searchJobID, &result)
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
			return nil, HTTPErrorWithInternal(ErrorFailedToDepsolve, fmt.Errorf("Search job %q timed out", searchJobID))
		default:
		}
	}

	return result.Packages, nil
}

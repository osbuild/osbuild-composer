package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type DepsolveJobImpl struct {
	RPMMDCache string
}

// depsolve each package set in the pacakgeSets map.  The repositories defined
// in repos are used for all package sets, whereas the repositories in
// packageSetsRepos are only used for the package set with the same name
// (matching map keys).
func (impl *DepsolveJobImpl) depsolve(packageSets map[string]rpmmd.PackageSet, repos []rpmmd.RepoConfig, packageSetsRepos map[string][]rpmmd.RepoConfig, modulePlatformID, arch, releasever string) (map[string][]rpmmd.PackageSpec, error) {
	rpmMD := rpmmd.NewRPMMD(impl.RPMMDCache)

	packageSpecs := make(map[string][]rpmmd.PackageSpec)
	for name, packageSet := range packageSets {
		repositories := make([]rpmmd.RepoConfig, len(repos))
		copy(repositories, repos)
		if packageSetRepositories, ok := packageSetsRepos[name]; ok {
			repositories = append(repositories, packageSetRepositories...)
		}
		packageSpec, _, err := rpmMD.Depsolve(packageSet, repositories, modulePlatformID, arch, releasever)
		if err != nil {
			return nil, err
		}
		packageSpecs[name] = packageSpec
	}
	return packageSpecs, nil
}

func (impl *DepsolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.DepsolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	var result worker.DepsolveJobResult
	result.PackageSpecs, err = impl.depsolve(args.PackageSets, args.Repos, args.PackageSetsRepos, args.ModulePlatformID, args.Arch, args.Releasever)
	if err != nil {
		switch e := err.(type) {
		case *rpmmd.DNFError:
			// Error originates from dnf-json (the http call dnf-json wasn't StatusOK)
			switch e.Kind {
			case "DepsolveError":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, err.Error())
			case "MarkingErrors":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFMarkingErrors, err.Error())
			default:
				// This still has the kind/reason format but a kind that's returned
				// by dnf-json and not explicitly handled here.
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFOtherError, err.Error())
				logWithId.Errorf("Unhandled dnf-json error in depsolve job: %v", err)
			}
		case error:
			// Error originates from internal/rpmmd, not from dnf-json
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorRPMMDError, err.Error())
			logWithId.Errorf("rpmmd error in depsolve job: %v", err)
		}
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

package main

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type DepsolveJobImpl struct {
	RPMMDCache string
}

func (impl *DepsolveJobImpl) depsolve(packageSets map[string]rpmmd.PackageSet, repos []rpmmd.RepoConfig, modulePlatformID, arch, releasever string, packageSetsRepositories map[string][]rpmmd.RepoConfig) (map[string][]rpmmd.PackageSpec, error) {
	rpmMD := rpmmd.NewRPMMD(impl.RPMMDCache)

	packageSpecs := make(map[string][]rpmmd.PackageSpec)
	for name, packageSet := range packageSets {
		repositories := make([]rpmmd.RepoConfig, len(repos))
		copy(repositories, repos)
		if packageSetRepositories, ok := packageSetsRepositories[name]; ok {
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
	var args worker.DepsolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	var result worker.DepsolveJobResult
	result.PackageSpecs, err = impl.depsolve(args.PackageSets, args.Repos, args.ModulePlatformID, args.Arch, args.Releasever, args.PackageSetsRepositories)
	if err != nil {
		switch e := err.(type) {
		case *rpmmd.DNFError:
			// Error originates from dnf-json (the http call dnf-json wasn't StatusOK)
			switch e.Kind {
			case "DepsolveError":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, err.Error())
			case "MarkingError":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFMarkingError, err.Error())
			default:
				// This still has the kind/reason format but a kind that's returned
				// by dnf-json and not explicitly handled here.
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFOtherError, err.Error())
			}
		case error:
			// Error originates from internal/rpmmd, not from dnf-json
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorRPMMDError, err.Error())
		}
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

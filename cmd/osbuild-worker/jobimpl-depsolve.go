package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/dnfjson"
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
func (impl *DepsolveJobImpl) depsolve(packageSetsChains map[string][]string, packageSets map[string]rpmmd.PackageSet, repos []rpmmd.RepoConfig, packageSetsRepos map[string][]rpmmd.RepoConfig, modulePlatformID, arch, releasever string) (map[string][]rpmmd.PackageSpec, error) {
	solver := dnfjson.NewSolver(modulePlatformID, releasever, arch, impl.RPMMDCache)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	psRepos := make([][]rpmmd.RepoConfig, 0)

	// first depsolve package sets that are part of a chain
	for specName, setNames := range packageSetsChains {
		pkgSets := make([]rpmmd.PackageSet, len(setNames))
		for idx, pkgSetName := range setNames {
			pkgSets[idx] = packageSets[pkgSetName]
			psRepos = append(psRepos, packageSetsRepos[pkgSetName]) // will be nil if it doesn't exist
			delete(packageSets, pkgSetName)                         // will be depsolved here: remove from map
		}
		res, err := solver.Depsolve(pkgSets, repos, psRepos)
		if err != nil {
			return nil, err
		}
		depsolvedSets[specName] = res.Dependencies
	}

	// depsolve the rest of the package sets
	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve([]rpmmd.PackageSet{pkgSet}, repos, [][]rpmmd.RepoConfig{packageSetsRepos[name]})
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res.Dependencies
	}
	return depsolvedSets, nil
}

func (impl *DepsolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.DepsolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	var result worker.DepsolveJobResult
	result.PackageSpecs, err = impl.depsolve(args.PackageSetsChains, args.PackageSets, args.Repos, args.PackageSetsRepos, args.ModulePlatformID, args.Arch, args.Releasever)
	if err != nil {
		switch e := err.(type) {
		case *dnfjson.Error:
			// Error originates from dnf-json
			switch e.Kind {
			case "DepsolveError":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, err.Error())
			case "MarkingErrors":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFMarkingErrors, err.Error())
			case "RepoError":
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFRepoError, err.Error())
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

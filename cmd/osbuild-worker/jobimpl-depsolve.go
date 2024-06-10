package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

// Used by both depsolve and osbuild jobs
type RepositoryMTLSConfig struct {
	BaseURL        *url.URL
	CA             string
	MTLSClientKey  string
	MTLSClientCert string
	Proxy          *url.URL
}

func (rmc *RepositoryMTLSConfig) CompareBaseURL(baseURLStr string) (bool, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return false, err
	}

	if baseURL.Scheme != rmc.BaseURL.Scheme {
		return false, nil
	}
	if baseURL.Host != rmc.BaseURL.Host {
		return false, nil
	}
	if !strings.HasPrefix(baseURL.Path, rmc.BaseURL.Path) {
		return false, nil
	}

	return true, nil
}

type DepsolveJobImpl struct {
	Solver               *dnfjson.BaseSolver
	RepositoryMTLSConfig *RepositoryMTLSConfig
}

// depsolve each package set in the pacakgeSets map.  The repositories defined
// in repos are used for all package sets, whereas the repositories in
// packageSetsRepos are only used for the package set with the same name
// (matching map keys).
func (impl *DepsolveJobImpl) depsolve(packageSets map[string][]rpmmd.PackageSet, modulePlatformID, arch, releasever string) (map[string][]rpmmd.PackageSpec, map[string][]rpmmd.RepoConfig, error) {
	solver := impl.Solver.NewWithConfig(modulePlatformID, releasever, arch, "")
	if impl.RepositoryMTLSConfig != nil && impl.RepositoryMTLSConfig.Proxy != nil {
		err := solver.SetProxy(impl.RepositoryMTLSConfig.Proxy.String())
		if err != nil {
			return nil, nil, err
		}
	}

	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	repoConfigs := make(map[string][]rpmmd.RepoConfig)
	for name, pkgSet := range packageSets {
		res, repos, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, nil, err
		}
		depsolvedSets[name] = res
		repoConfigs[name] = repos
	}

	return depsolvedSets, repoConfigs, nil
}

func workerClientErrorFrom(err error) (*clienterrors.Error, error) {
	switch e := err.(type) {
	case dnfjson.Error:
		// Error originates from dnf-json
		switch e.Kind {
		case "DepsolveError":
			return clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, err.Error(), e.Reason), nil
		case "MarkingErrors":
			return clienterrors.WorkerClientError(clienterrors.ErrorDNFMarkingErrors, err.Error(), e.Reason), nil
		case "RepoError":
			return clienterrors.WorkerClientError(clienterrors.ErrorDNFRepoError, err.Error(), e.Reason), nil
		default:
			err := fmt.Errorf("Unhandled dnf-json error in depsolve job: %v", err)
			// This still has the kind/reason format but a kind that's returned
			// by dnf-json and not explicitly handled here.
			return clienterrors.WorkerClientError(clienterrors.ErrorDNFOtherError, err.Error(), e.Reason), err
		}
	default:
		err := fmt.Errorf("rpmmd error in depsolve job: %v", err)
		// Error originates from internal/rpmmd, not from dnf-json
		return clienterrors.WorkerClientError(clienterrors.ErrorRPMMDError, err.Error(), nil), err
	}
}

func (impl *DepsolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.DepsolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	var result worker.DepsolveJobResult

	if impl.RepositoryMTLSConfig != nil {
		for pkgsetsi, pkgsets := range args.PackageSets {
			for pkgseti, pkgset := range pkgsets {
				for repoi, repo := range pkgset.Repositories {
					for _, baseurlstr := range repo.BaseURLs {
						match, err := impl.RepositoryMTLSConfig.CompareBaseURL(baseurlstr)
						if err != nil {
							result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidRepositoryURL, "Repository URL is malformed", err.Error())
							return err
						}
						if match {
							args.PackageSets[pkgsetsi][pkgseti].Repositories[repoi].SSLCACert = impl.RepositoryMTLSConfig.CA
							args.PackageSets[pkgsetsi][pkgseti].Repositories[repoi].SSLClientKey = impl.RepositoryMTLSConfig.MTLSClientKey
							args.PackageSets[pkgsetsi][pkgseti].Repositories[repoi].SSLClientCert = impl.RepositoryMTLSConfig.MTLSClientCert
						}
					}
				}
			}
		}
	}

	result.PackageSpecs, result.RepoConfigs, err = impl.depsolve(args.PackageSets, args.ModulePlatformID, args.Arch, args.Releasever)
	if err != nil {
		result.JobError, err = workerClientErrorFrom(err)
		if err != nil {
			logWithId.Errorf(err.Error())
		}
	}
	if err := impl.Solver.CleanCache(); err != nil {
		// log and ignore
		logWithId.Errorf("Error during rpm repo cache cleanup: %s", err.Error())
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

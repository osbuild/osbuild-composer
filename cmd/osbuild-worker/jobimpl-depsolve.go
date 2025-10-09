package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
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

// SetupRepoSSL copies the CA, Key, and Cert to the RepoConfig
func (rmc *RepositoryMTLSConfig) SetupRepoSSL(repo *rpmmd.RepoConfig) {
	repo.SSLCACert = rmc.CA
	repo.SSLClientKey = rmc.MTLSClientKey
	repo.SSLClientCert = rmc.MTLSClientCert
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
	Solver               *depsolvednf.BaseSolver
	RepositoryMTLSConfig *RepositoryMTLSConfig
}

// depsolve each package set in the pacakgeSets map.  The repositories defined
// in repos are used for all package sets, whereas the repositories in
// packageSetsRepos are only used for the package set with the same name
// (matching map keys).
func (impl *DepsolveJobImpl) depsolve(packageSets map[string][]rpmmd.PackageSet, modulePlatformID, arch, releasever string, sbomType sbom.StandardType) (map[string]worker.DepsolvedPackageList, map[string][]rpmmd.RepoConfig, map[string]worker.SbomDoc, error) {
	solver := impl.Solver.NewWithConfig(modulePlatformID, releasever, arch, "")
	if impl.RepositoryMTLSConfig != nil && impl.RepositoryMTLSConfig.Proxy != nil {
		err := solver.SetProxy(impl.RepositoryMTLSConfig.Proxy.String())
		if err != nil {
			return nil, nil, nil, err
		}
	}

	depsolvedSets := make(map[string]worker.DepsolvedPackageList)
	repoConfigs := make(map[string][]rpmmd.RepoConfig)
	var sbomDocs map[string]worker.SbomDoc
	if sbomType != sbom.StandardTypeNone {
		sbomDocs = make(map[string]worker.SbomDoc)
	}
	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet, sbomType)
		if err != nil {
			return nil, nil, nil, err
		}
		depsolvedSets[name] = worker.DepsolvedPackageListFromRPMMDList(res.Packages)
		repoConfigs[name] = res.Repos
		if sbomType != sbom.StandardTypeNone {
			sbomDocs[name] = worker.SbomDoc(*res.SBOM)
		}
	}

	return depsolvedSets, repoConfigs, sbomDocs, nil
}

func workerClientErrorFrom(err error, logWithId *logrus.Entry) *clienterrors.Error {
	if err == nil {
		logWithId.Errorf("workerClientErrorFrom expected an error to be processed. Not nil")
	}

	var dnfjsonErr depsolvednf.Error
	if errors.As(err, &dnfjsonErr) {
		// Error originates from dnf-json
		reason := fmt.Sprintf("DNF error occurred: %s", dnfjsonErr.Kind)
		details := dnfjsonErr.Reason
		switch dnfjsonErr.Kind {
		case "DepsolveError":
			return clienterrors.New(clienterrors.ErrorDNFDepsolveError, reason, details)
		case "MarkingErrors":
			return clienterrors.New(clienterrors.ErrorDNFMarkingErrors, reason, details)
		case "RepoError":
			return clienterrors.New(clienterrors.ErrorDNFRepoError, reason, details)
		default:
			logWithId.Errorf("Unhandled dnf-json error in depsolve job: %v", err)
			// This still has the kind/reason format but a kind that's returned
			// by dnf-json and not explicitly handled here.
			return clienterrors.New(clienterrors.ErrorDNFOtherError, reason, details)
		}
	} else {
		reason := "rpmmd error in depsolve job"
		details := fmt.Sprintf("%v", err)
		// Error originates from internal/rpmmd, not from dnf-json
		//
		// XXX: it seems slightly dangerous to assume that any
		// "error" we get there is coming from rpmmd, can we
		// generate a more typed error from dnfjson here for
		// rpmmd errors?
		return clienterrors.New(clienterrors.ErrorRPMMDError, reason, details)
	}
}

func (impl *DepsolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())

	var result worker.DepsolveJobResult
	// ALWAYS return a result
	defer func() {
		err := job.Update(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.DepsolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	if impl.RepositoryMTLSConfig != nil {
		for pkgsetsi, pkgsets := range args.PackageSets {
			for pkgseti, pkgset := range pkgsets {
				for repoi, repo := range pkgset.Repositories {
					for _, baseurlstr := range repo.BaseURLs {
						match, err := impl.RepositoryMTLSConfig.CompareBaseURL(baseurlstr)
						if err != nil {
							result.JobError = clienterrors.New(clienterrors.ErrorInvalidRepositoryURL, "Repository URL is malformed", err.Error())
							return err
						}
						if match {
							impl.RepositoryMTLSConfig.SetupRepoSSL(&args.PackageSets[pkgsetsi][pkgseti].Repositories[repoi])
						}
					}
				}
			}
		}
	}

	result.PackageSpecs, result.RepoConfigs, result.SbomDocs, err = impl.depsolve(args.PackageSets, args.ModulePlatformID, args.Arch, args.Releasever, args.SbomType)
	if err != nil {
		result.JobError = workerClientErrorFrom(err, logWithId)
	}
	if err := impl.Solver.CleanCache(); err != nil {
		// log and ignore
		logWithId.Errorf("Error during rpm repo cache cleanup: %s", err.Error())
	}

	// NOTE: result is returned by deferred function above

	return nil
}

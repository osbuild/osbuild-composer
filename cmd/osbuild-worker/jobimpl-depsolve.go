package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/sbom"
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
	return compareURL(rmc.BaseURL, baseURL)
}

// Used by both depsolve and osbuild jobs
type RepositoryBasicAuthConfig struct {
	BaseURL      *url.URL
	UsernamePath string
	PasswordPath string
	Proxy        *url.URL
}

func (rbc *RepositoryBasicAuthConfig) Username() (string, error) {
	user, err := os.ReadFile(rbc.UsernamePath)
	if err != nil {
		return "", err
	}
	return string(user), nil
}

func (rbc *RepositoryBasicAuthConfig) Password() (string, error) {
	pwd, err := os.ReadFile(rbc.PasswordPath)
	if err != nil {
		return "", err
	}
	return string(pwd), nil
}

func (rbc *RepositoryBasicAuthConfig) CompareBaseURL(baseURLStr string) (bool, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return false, err
	}
	return compareURL(rbc.BaseURL, baseURL)
}

func (rbc *RepositoryBasicAuthConfig) SetupRepoBasicAuth(repo *rpmmd.RepoConfig) error {
	username, err := rbc.Username()
	if err != nil {
		return err
	}
	repo.Username = username
	password, err := rbc.Password()
	if err != nil {
		return err
	}
	repo.Password = password
	return nil
}

func compareURL(urlA, urlB *url.URL) (bool, error) {
	if urlA.Scheme != urlB.Scheme {
		return false, nil
	}
	if urlA.Host != urlB.Host {
		return false, nil
	}
	if !strings.HasPrefix(urlA.Path, urlB.Path) {
		return false, nil
	}

	return true, nil
}

type DepsolveJobImpl struct {
	Solver                    *depsolvednf.BaseSolver
	RepositoryMTLSConfig      *RepositoryMTLSConfig
	RepositoryBasicAuthConfig *RepositoryBasicAuthConfig
}

// depsolve each package set in the packageSets map. The repositories defined
// in repos are used for all package sets, whereas the repositories in
// packageSetsRepos are only used for the package set with the same name
// (matching map keys).
//
// On error, returns a partial result with JobError set.
func (impl *DepsolveJobImpl) depsolve(packageSets map[string][]rpmmd.PackageSet, modulePlatformID, arch, releasever string, sbomType sbom.StandardType, logWithId *logrus.Entry) *worker.DepsolveJobResult {
	result := &worker.DepsolveJobResult{}

	solver := impl.Solver.NewWithConfig(modulePlatformID, releasever, arch, "")
	if impl.RepositoryMTLSConfig != nil && impl.RepositoryMTLSConfig.Proxy != nil {
		err := solver.SetProxy(impl.RepositoryMTLSConfig.Proxy.String())
		if err != nil {
			result.JobError = workerClientErrorFrom(err, logWithId)
			return result
		}
	}
	if impl.RepositoryBasicAuthConfig != nil && impl.RepositoryBasicAuthConfig.Proxy != nil {
		err := solver.SetProxy(impl.RepositoryBasicAuthConfig.Proxy.String())
		if err != nil {
			result.JobError = workerClientErrorFrom(err, logWithId)
			return result
		}
	}

	depsolvedSets := make(map[string]worker.DepsolvedPackageList)
	transactions := make(map[string][]worker.DepsolvedPackageList)
	repoConfigs := make(map[string][]worker.DepsolvedRepoConfig)
	modules := make(map[string][]worker.DepsolvedModuleSpec)
	var solverName string
	var sbomDocs map[string]worker.SbomDoc
	if sbomType != sbom.StandardTypeNone {
		sbomDocs = make(map[string]worker.SbomDoc)
	}

	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet, sbomType)
		if err != nil {
			result.JobError = workerClientErrorFrom(err, logWithId)
			return result
		}
		if solverName == "" {
			solverName = res.Solver
		}
		// NOTE: PackageSpecs is populated for backward compatibility with older osbuild-composer servers
		// that don't recognize the Transactions field in the DepsolveJobResult yet.
		// TODO (2026-02-12, thozza): remove PackageSpecs after ~ 1 month / 2 releases
		depsolvedSets[name] = worker.DepsolvedPackageListFromRPMMDList(res.Transactions.AllPackages())
		transactions[name] = worker.DepsolvedTransactionsFromRPMMD(res.Transactions)
		repoConfigs[name] = worker.DepsolvedRepoConfigListFromRPMMDList(res.Repos)
		if len(res.Modules) > 0 {
			modules[name] = worker.DepsolvedModuleSpecListFromRPMMDList(res.Modules)
		}
		if sbomType != sbom.StandardTypeNone {
			sbomDocs[name] = worker.SbomDoc(*res.SBOM)
		}
	}

	result.PackageSpecs = depsolvedSets
	result.Transactions = transactions
	result.RepoConfigs = repoConfigs
	result.SbomDocs = sbomDocs
	result.Modules = modules
	result.Solver = solverName

	return result
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
		err := job.Finish(&result)
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

	if impl.RepositoryBasicAuthConfig != nil {
		for pkgsetsi, pkgsets := range args.PackageSets {
			for pkgseti, pkgset := range pkgsets {
				for repoi, repo := range pkgset.Repositories {
					for _, baseurlstr := range repo.BaseURLs {
						match, err := impl.RepositoryBasicAuthConfig.CompareBaseURL(baseurlstr)
						if err != nil {
							result.JobError = clienterrors.New(clienterrors.ErrorInvalidRepositoryURL, "Repository URL is malformed", err.Error())
							return err
						}
						if match {
							err = impl.RepositoryBasicAuthConfig.SetupRepoBasicAuth(&args.PackageSets[pkgsetsi][pkgseti].Repositories[repoi])
							if err != nil {
								result.JobError = clienterrors.New(
									clienterrors.ErrorInvalidConfig,
									fmt.Sprintf("Adding basic authentication for repository (%s) failed", baseurlstr),
									err.Error(),
								)
								return err
							}
						}
					}
				}
			}
		}
	}

	depsolveResult := impl.depsolve(args.PackageSets, args.ModulePlatformID, args.Arch, args.Releasever, args.SbomType, logWithId)
	result = *depsolveResult

	if err := impl.Solver.CleanCache(); err != nil {
		// log and ignore
		logWithId.Errorf("Error during rpm repo cache cleanup: %s", err.Error())
	}

	// NOTE: result is returned by deferred function above

	return nil
}

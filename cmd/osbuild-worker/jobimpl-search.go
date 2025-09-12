package main

import (
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/sirupsen/logrus"
)

// SearchPackagesJobImpl shares the solver with the depsolve job.
type SearchPackagesJobImpl struct {
	Solver               *depsolvednf.BaseSolver
	RepositoryMTLSConfig *RepositoryMTLSConfig
}

func (impl *SearchPackagesJobImpl) search(repos []rpmmd.RepoConfig, modulePlatformID, arch, releasever string, packages []string) (rpmmd.PackageList, error) {
	solver := impl.Solver.NewWithConfig(modulePlatformID, releasever, arch, "")
	if impl.RepositoryMTLSConfig != nil && impl.RepositoryMTLSConfig.Proxy != nil {
		err := solver.SetProxy(impl.RepositoryMTLSConfig.Proxy.String())
		if err != nil {
			return nil, err
		}
	}

	return solver.SearchMetadata(repos, packages)
}

// Run executes the search and returns the results
func (impl *SearchPackagesJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())

	var result worker.SearchPackagesJobResult
	// ALWAYS return a result
	defer func() {
		err := job.Update(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.SearchPackagesJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	if impl.RepositoryMTLSConfig != nil {
		for repoi, repo := range args.Repositories {
			for _, baseurlstr := range repo.BaseURLs {
				match, err := impl.RepositoryMTLSConfig.CompareBaseURL(baseurlstr)
				if err != nil {
					result.JobError = clienterrors.New(clienterrors.ErrorInvalidRepositoryURL, "Repository URL is malformed", err.Error())
					return err
				}
				if match {
					impl.RepositoryMTLSConfig.SetupRepoSSL(&args.Repositories[repoi])
				}
			}
		}
	}

	result.Packages, err = impl.search(args.Repositories, args.ModulePlatformID, args.Arch, args.Releasever, args.Packages)
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

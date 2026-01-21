package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"

	"github.com/sirupsen/logrus"

	repos "github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type BootcManifestJobImpl struct {
	Store string
}


// This job generates a bootc manifest and produces a result which is compatible with the OSBuild
// job. The manifest is generated on the worker because instantiating a bootc distro requires
// running and inspecting the container (this is done within images).
//
// Contrary to ManifestJobByID, this does not require any resolve jobs. So the jobs should simple be
// BootcManifestJob -> OSBuildJob.
func (impl *BootcManifestJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.BootcManifestJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	result := worker.BootcManifestJobResult{
		Manifest: nil,
		ManifestInfo: worker.ManifestInfo{
			OSBuildComposerVersion: common.BuildVersion(),
		},
	}

	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic: %v", r)
			logWithId.Errorf("%s", debug.Stack())

			result.JobError = clienterrors.New(
				clienterrors.ErrorJobPanicked,
				fmt.Sprintf("job panicked:\n%v\n\noriginal error:\n%v", r, result.JobError),
				nil,
			)
		}
		err := job.Finish(result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	logWithId.Infof("Generating bootc manifest (image ref: %s)", args.Ref)
	distri, err := bootc.NewBootcDistro(args.Ref, &bootc.DistroOptions{})
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}
	if err := distri.SetBuildContainer(args.BuildRef); err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}
	archi, err := distri.GetArch(args.Arch)
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}

	imgType, err := archi.GetImageType(args.ImageType)
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}

	repos, err := reporegistry.New(nil, []fs.FS{repos.FS})
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}

	rpmDownloader := osbuild.RpmDownloader(osbuild.RpmDownloaderLibrepo)
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Cachedir: impl.Store,
		// XXX: hack to skip repo loading for the bootc image.
		// We need to add a SkipRepositories or similar to
		// manifestgen instead to make this clean
		OverrideRepos: []rpmmd.RepoConfig{
			{
				BaseURLs: []string{"https://example.com/not-used"},
			},
		},
		RpmDownloader: rpmDownloader,
		Depsolve: func(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
			depsolveResult, err := manifestgen.DefaultDepsolve(solver, cacheDir, depsolveWarningsOutput, packageSets, d, arch)
			// extracting needs to happen while container is mounted
			depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
			for k, v := range depsolveResult {
				depsolvedRepos[k] = v.Repos
			}
			return depsolveResult, err
		},
		// this turns (blueprint validation) warnings into
		// warnings as they are visible to the user
		WarningsOutput: os.Stderr,
	})
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}
	manifest, err := mg.Generate(&args.Blueprint, imgType, nil)
	if err != nil {
		reason := fmt.Sprintf("Unable to generate bootc manifest: %s", err.Error())
		result.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, nil)
		return err
	}
	result.Manifest = manifest

	pipelineNames, err := parseManifestPipelines(manifest)
	if err != nil {
		// Not a critical failure. Log the error and continue. This will cause
		// the job log to be unordered.
		// For Koji builds, it will also fail to create package listings.
		logWithId.Warningf("failed to parse manifest for pipeline names: %v", err)
	}
	result.ManifestInfo.PipelineNames = pipelineNames

	return nil
}

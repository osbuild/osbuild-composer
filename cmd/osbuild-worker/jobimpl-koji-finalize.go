package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type KojiFinalizeJobImpl struct {
	KojiServers map[string]koji.Credentials
}

func (impl *KojiFinalizeJobImpl) kojiImport(
	server string,
	build koji.ImageBuild,
	buildRoots []koji.BuildRoot,
	images []koji.Image,
	directory, token string) error {
	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := creds.NewKojiFromCreds(server)
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	_, err = k.CGImport(build, buildRoots, images, directory, token)
	if err != nil {
		return fmt.Errorf("Could not import build into koji: %v", err)
	}

	return nil
}

func (impl *KojiFinalizeJobImpl) kojiFail(server string, buildID int, token string) error {
	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := creds.NewKojiFromCreds(server)
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	return k.CGFailBuild(buildID, token)
}

func (impl *KojiFinalizeJobImpl) Run(job worker.Job) error {
	var args worker.KojiFinalizeJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	initArgs, osbuildKojiResults, err := extractDynamicArgs(job)
	if err != nil {
		return err
	}

	// Check the dependencies early. Fail the koji build if any of them failed.
	if hasFailedDependency(*initArgs, osbuildKojiResults) {
		err = impl.kojiFail(args.Server, int(initArgs.BuildID), initArgs.Token)

		// Update the status immediately and bail out.
		var result worker.KojiFinalizeJobResult
		if err != nil {
			result.KojiError = err.Error()
		}
		err = job.Update(&result)
		if err != nil {
			return fmt.Errorf("Error reporting job result: %v", err)
		}
		return nil
	}

	build := koji.ImageBuild{
		BuildID:   initArgs.BuildID,
		TaskID:    args.TaskID,
		Name:      args.Name,
		Version:   args.Version,
		Release:   args.Release,
		StartTime: int64(args.StartTime),
		EndTime:   time.Now().Unix(),
	}

	var buildRoots []koji.BuildRoot
	var images []koji.Image
	for i, buildArgs := range osbuildKojiResults {
		buildRoots = append(buildRoots, koji.BuildRoot{
			ID: uint64(i),
			Host: koji.Host{
				Os:   buildArgs.HostOS,
				Arch: buildArgs.Arch,
			},
			ContentGenerator: koji.ContentGenerator{
				Name:    "osbuild",
				Version: "0", // TODO: put the correct version here
			},
			Container: koji.Container{
				Type: "none",
				Arch: buildArgs.Arch,
			},
			Tools: []koji.Tool{},
			RPMs:  rpmmd.OSBuildStagesToRPMs(buildArgs.OSBuildOutput.Build.Stages),
		})
		images = append(images, koji.Image{
			BuildRootID:  uint64(i),
			Filename:     args.KojiFilenames[i],
			FileSize:     buildArgs.ImageSize,
			Arch:         buildArgs.Arch,
			ChecksumType: "md5",
			MD5:          buildArgs.ImageHash,
			Type:         "image",
			RPMs:         rpmmd.OSBuildStagesToRPMs(buildArgs.OSBuildOutput.Stages),
			Extra: koji.ImageExtra{
				Info: koji.ImageExtraInfo{
					Arch: buildArgs.Arch,
				},
			},
		})
	}

	var result worker.KojiFinalizeJobResult
	err = impl.kojiImport(args.Server, build, buildRoots, images, args.KojiDirectory, initArgs.Token)
	if err != nil {
		result.KojiError = err.Error()
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

// Extracts dynamic args of the koji-finalize job. Returns an error if they
// cannot be unmarshalled.
func extractDynamicArgs(job worker.Job) (*worker.KojiInitJobResult, []worker.OSBuildKojiJobResult, error) {
	var kojiInitResult worker.KojiInitJobResult
	err := job.DynamicArgs(0, &kojiInitResult)
	if err != nil {
		return nil, nil, err
	}

	osbuildKojiResults := make([]worker.OSBuildKojiJobResult, job.NDynamicArgs()-1)

	for i := 1; i < job.NDynamicArgs(); i++ {
		err = job.DynamicArgs(i, &osbuildKojiResults[i-1])
		if err != nil {
			return nil, nil, err
		}
	}

	return &kojiInitResult, osbuildKojiResults, nil
}

// Returns true if any of koji-finalize dependencies failed.
func hasFailedDependency(kojiInitResult worker.KojiInitJobResult, osbuildKojiResults []worker.OSBuildKojiJobResult) bool {
	if kojiInitResult.KojiError != "" {
		return true
	}

	for _, r := range osbuildKojiResults {
		if !r.OSBuildOutput.Success || r.KojiError != "" {
			return true
		}
	}
	return false
}

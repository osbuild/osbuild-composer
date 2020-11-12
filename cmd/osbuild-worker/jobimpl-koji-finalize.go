package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type KojiFinalizeJobImpl struct {
	KojiServers map[string]koji.GSSAPICredentials
}

func (impl *KojiFinalizeJobImpl) kojiImport(
	server string,
	build koji.ImageBuild,
	buildRoots []koji.BuildRoot,
	images []koji.Image,
	directory, token string) error {
	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http transport and enable renegotiation.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &creds, transport)
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
	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http transport and enable renegotiation.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &creds, transport)
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

	var failure bool

	var initArgs worker.KojiInitJobResult
	err = job.DynamicArgs(0, &initArgs)
	if err != nil {
		return err
	}

	if initArgs.KojiError != "" {
		failure = true
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
	for i := 1; i < job.NDynamicArgs(); i++ {
		var buildArgs worker.OSBuildKojiJobResult
		err = job.DynamicArgs(i, &buildArgs)
		if err != nil {
			return err
		}
		if !buildArgs.OSBuildOutput.Success || buildArgs.KojiError != "" {
			failure = true
			break
		}
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
			RPMs:  osbuildStagesToRPMs(buildArgs.OSBuildOutput.Build.Stages),
		})
		images = append(images, koji.Image{
			BuildRootID:  uint64(i),
			Filename:     args.KojiFilenames[i-1],
			FileSize:     buildArgs.ImageSize,
			Arch:         buildArgs.Arch,
			ChecksumType: "md5",
			MD5:          buildArgs.ImageHash,
			Type:         "image",
			RPMs:         osbuildStagesToRPMs(buildArgs.OSBuildOutput.Stages),
			Extra: koji.ImageExtra{
				Info: koji.ImageExtraInfo{
					Arch: buildArgs.Arch,
				},
			},
		})
	}

	var result worker.KojiFinalizeJobResult
	if failure {
		err = impl.kojiFail(args.Server, int(initArgs.BuildID), initArgs.Token)
		if err != nil {
			result.KojiError = err.Error()
		}
	} else {
		err = impl.kojiImport(args.Server, build, buildRoots, images, args.KojiDirectory, initArgs.Token)
		if err != nil {
			result.KojiError = err.Error()
		}
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

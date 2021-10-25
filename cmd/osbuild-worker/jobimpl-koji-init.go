package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type KojiInitJobImpl struct {
	KojiServers map[string]koji.Credentials
}

func (impl *KojiInitJobImpl) kojiInit(server, name, version, release string) (string, uint64, error) {
	serverURL, err := url.Parse(server)
	if err != nil {
		return "", 0, err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return "", 0, fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := creds.NewKojiFromCreds(server)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	buildInfo, err := k.CGInitBuild(name, version, release)
	if err != nil {
		return "", 0, err
	}

	return buildInfo.Token, uint64(buildInfo.BuildID), nil
}

func (impl *KojiInitJobImpl) Run(job worker.Job) error {
	var args worker.KojiInitJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	var result worker.KojiInitJobResult
	result.Token, result.BuildID, err = impl.kojiInit(args.Server, args.Name, args.Version, args.Release)
	if err != nil {
		result.KojiError = err.Error()
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

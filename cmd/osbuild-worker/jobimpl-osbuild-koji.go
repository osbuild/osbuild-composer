package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type OSBuildKojiJobImpl struct {
	Store       string
	Output      string
	KojiServers map[string]koji.GSSAPICredentials
}

func (impl *OSBuildKojiJobImpl) kojiUpload(file *os.File, server, directory, filename string) (string, uint64, error) {
	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http transport and enable renegotiation.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return "", 0, err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return "", 0, fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &creds, transport)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	return k.Upload(file, directory, filename)
}

func (impl *OSBuildKojiJobImpl) Run(job worker.Job) error {
	outputDirectory, err := ioutil.TempDir(impl.Output, job.Id().String()+"-*")
	if err != nil {
		return fmt.Errorf("error creating temporary output directory: %v", err)
	}
	defer func() {
		err := os.RemoveAll(outputDirectory)
		if err != nil {
			log.Printf("Error removing temporary output directory (%s): %v", outputDirectory, err)
		}
	}()

	var args worker.OSBuildKojiJob
	err = job.Args(&args)
	if err != nil {
		return err
	}

	var initArgs worker.KojiInitJobResult
	err = job.DynamicArgs(0, &initArgs)
	if err != nil {
		return err
	}

	var result worker.OSBuildKojiJobResult
	result.Arch = common.CurrentArch()
	result.HostOS, err = distro.GetRedHatRelease()
	if err != nil {
		return err
	}

	exports := []string{"assembler"} // NOTE: Koji API doesn't support new image types yet
	if initArgs.KojiError == "" {
		result.OSBuildOutput, err = RunOSBuild(args.Manifest, impl.Store, outputDirectory, exports, os.Stderr)
		if err != nil {
			return err
		}

		if result.OSBuildOutput.Success {
			f, err := os.Open(path.Join(outputDirectory, args.ImageName))
			if err != nil {
				return err
			}
			result.ImageHash, result.ImageSize, err = impl.kojiUpload(f, args.KojiServer, args.KojiDirectory, args.KojiFilename)
			if err != nil {
				result.KojiError = err.Error()
			}
		}
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

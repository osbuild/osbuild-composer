package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type OSBuildKojiJobImpl struct {
	Store       string
	Output      string
	KojiServers map[string]kojiServer
}

func (impl *OSBuildKojiJobImpl) kojiUpload(file *os.File, server, directory, filename string) (string, uint64, error) {

	serverURL, err := url.Parse(server)
	if err != nil {
		return "", 0, err
	}

	kojiServer, exists := impl.KojiServers[serverURL.Hostname()]
	transport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor)
	if !exists {
		return "", 0, fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &kojiServer.creds, transport)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			logrus.Warnf("koji logout failed: %v", err)
		}
	}()

	return k.Upload(file, directory, filename)
}

func validateKojiResult(result *worker.OSBuildKojiJobResult, jobID string) {
	logWithId := logrus.WithField("jobId", jobID)
	if result.JobError != nil {
		logWithId.Errorf("osbuild job failed: %s", result.JobError.Reason)
		return
	}
	// if the job failed, but the JobError is
	// nil, we still need to handle this as an error
	if result.OSBuildOutput == nil || !result.OSBuildOutput.Success {
		reason := "osbuild job was unsuccessful"
		logWithId.Errorf("osbuild job failed: %s", reason)
		result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, reason)
	} else {
		logWithId.Infof("osbuild-koji job succeeded")
	}
}

func (impl *OSBuildKojiJobImpl) Run(job worker.Job) error {
	var result worker.OSBuildKojiJobResult
	outputDirectory, err := ioutil.TempDir(impl.Output, job.Id().String()+"-*")
	if err != nil {
		return fmt.Errorf("error creating temporary output directory: %v", err)
	}
	defer func() {
		validateKojiResult(&result, job.Id().String())

		// this is necessary for early return errors
		err = job.Update(&result)
		if err != nil {
			logrus.Warnf("Error reporting job result: %v", err)
		}

		err := os.RemoveAll(outputDirectory)
		if err != nil {
			logrus.Warnf("Error removing temporary output directory (%s): %v", outputDirectory, err)
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

	result.Arch = common.CurrentArch()
	result.HostOS, err = common.GetRedHatRelease()
	if err != nil {
		return err
	}

	// In case the manifest is empty, try to get it from dynamic args
	if len(args.Manifest) == 0 {
		if job.NDynamicArgs() > 1 {
			var manifestJR worker.ManifestJobByIDResult
			err = job.DynamicArgs(1, &manifestJR)
			if err != nil {
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args")
				return err
			}

			// skip the job if the manifest generation failed
			if manifestJR.JobError != nil {
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorManifestDependency, "Manifest dependency failed")
				return nil
			}
			args.Manifest = manifestJR.Manifest
			if len(args.Manifest) == 0 {
				err := fmt.Errorf("Received empty manifest")
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyManifest, err.Error())
				return err
			}
		} else {
			err := fmt.Errorf("Job has no manifest")
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyManifest, err.Error())
			return err
		}
	}

	if initArgs.JobError == nil {
		exports := args.Exports
		if len(exports) == 0 {
			// job did not define exports, likely coming from an older version of composer
			// fall back to default "assembler"
			exports = []string{"assembler"}
		} else if len(exports) > 1 {
			// this worker only supports returning one (1) export
			err = fmt.Errorf("at most one build artifact can be exported")
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, err.Error())
			return err
		}
		result.OSBuildOutput, err = osbuild.RunOSBuild(args.Manifest, impl.Store, outputDirectory, exports, nil, true, os.Stderr)
		if err != nil {
			return err
		}

		// NOTE: Currently OSBuild supports multiple exports, but this isn't used
		// by any of the image types and it can't be specified during the request.
		// Use the first (and presumably only) export for the imagePath.
		exportPath := exports[0]
		if result.OSBuildOutput.Success {
			f, err := os.Open(path.Join(outputDirectory, exportPath, args.ImageName))
			if err != nil {
				return err
			}
			result.ImageHash, result.ImageSize, err = impl.kojiUpload(f, args.KojiServer, args.KojiDirectory, args.KojiFilename)
			if err != nil {
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiBuild, err.Error())
			}
		}
	}

	// copy pipeline info to the result
	result.PipelineNames = args.PipelineNames

	return nil
}

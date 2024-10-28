package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type BootcManifestJobImpl struct{}

type ErrorInvalidImageType string

func (e ErrorInvalidImageType) Error() string { return string(e) }

type ErrorCommandExecution string

func (e ErrorCommandExecution) Error() string { return string(e) }

type ErrorInvalidArguments string

func (e ErrorInvalidArguments) Error() string { return string(e) }

type ErrorUnknown string

func (e ErrorUnknown) Error() string { return string(e) }

func setResponseError(err error, result *worker.BootcManifestJobResult) {
	switch e := err.(type) {
	case ErrorInvalidImageType:
		result.JobError = clienterrors.New(
			clienterrors.ErrorInvalidImageType,
			"Unsupported image type",
			e.Error(),
		)
	case ErrorCommandExecution:
		result.JobError = clienterrors.New(
			clienterrors.ErrorBootcManifestExec,
			"Error running bootc-image-builder command",
			e.Error(),
		)
	default:
		result.JobError = clienterrors.New(
			clienterrors.ErrorInvalidManifestArgs,
			"Invalid bootc parameters or parameter combination",
			err.Error(),
		)
	}
}

func buildManifestCommand(args *worker.BootcManifestJob) *exec.Cmd {
	baseArgs := []string{
		"sudo", "podman", "run",
		"--privileged",
		"--rm",
		"--pull=missing",
		"--storage-driver=vfs",
		"--cgroup-manager=cgroupfs",
		"--runtime=runc",
		"--security-opt=label=type:unconfined_t",
		"quay.io/centos-bootc/bootc-image-builder:latest",
		"manifest",
		args.ImageRef,
	}

	// Append optional arguments
	if args.ImageType != "" {
		baseArgs = append(baseArgs, "--type", args.ImageType)
	}
	if args.Arch != "" {
		baseArgs = append(baseArgs, "--target-arch", args.Arch)
	}
	if !args.TLSVerify {
		baseArgs = append(baseArgs, "--tls-verify=false")
	}

	return exec.Command(baseArgs[0], baseArgs[1:]...)
}

func (impl *BootcManifestJobImpl) Run(job worker.Job) error {
	logrus.Info("Running bootc-image-builder job")
	logWithId := logrus.WithField("jobId", job.Id())
	// Parse the job arguments
	var args worker.BootcManifestJob
	err := job.Args(&args)
	if err != nil {
		return err
	}
	jobResult := worker.BootcManifestJobResult{}

	cmd := buildManifestCommand(&args)
	logWithId.Infof("Running bib manifest: %s", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		logWithId.Errorf("Error running bootc-image-builder: %v, Output: %s", err, string(output))
		setResponseError(ErrorCommandExecution(err.Error()), &jobResult)
		return job.Update(&jobResult)
	}

	outputStr := string(output)
	re := regexp.MustCompile(`\{.*\}`)
	jsonData := re.FindString(outputStr)

	var manifestData manifest.OSBuildManifest
	err = json.Unmarshal([]byte(jsonData), &manifestData)
	if err != nil {
		logWithId.Errorf("Error unmarshalling manifest data: %v", err)
		setResponseError(ErrorCommandExecution(err.Error()), &jobResult)
	}

	jobResult.Manifest = manifestData
	jobResult.JobResult = worker.JobResult{}
	err = job.Update(&jobResult)
	if err != nil {
		setResponseError(ErrorUnknown(fmt.Sprintf("Error reporting job result: %v", err)), &jobResult)
		return job.Update(&jobResult)
	}

	return nil
}

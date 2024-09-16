package main

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type BootcImageBuilderJobImpl struct{}

type ErrorInvalidImageType string

func (e ErrorInvalidImageType) Error() string { return string(e) }

type ErrorCommandExecution string

func (e ErrorCommandExecution) Error() string { return string(e) }

type ErrorInvalidArguments string

func (e ErrorInvalidArguments) Error() string { return string(e) }

type ErrorUnknown string

func (e ErrorUnknown) Error() string { return string(e) }

func setResponseError(err error, result *worker.BootcImageBuilderJobResult) {
	switch e := err.(type) {
	case ErrorInvalidImageType:
		result.JobError = clienterrors.New(
			clienterrors.ErrorInvalidImageType,
			"Unsupported image type",
			e.Error(),
		)
	case ErrorCommandExecution:
		result.JobError = clienterrors.New(
			clienterrors.ErrorBootcExecution,
			"Error running bootc-image-builder command",
			e.Error(),
		)
	default:
		result.JobError = clienterrors.New(
			clienterrors.ErrorInvalidArguments,
			"Invalid bootc parameters or parameter combination",
			err.Error(),
		)
	}
}

func (impl *BootcImageBuilderJobImpl) Run(job worker.Job) error {
	logrus.Info("Running bootc-image-builder job")
	logWithId := logrus.WithField("jobId", job.Id())
	// Parse the job arguments
	var args worker.BootcImageBuilderJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	// Initialize the result
	jobResult := worker.BootcImageBuilderJobResult{}

	// Determine the output image type
	imageType := ""
	switch args.ImageType {
	case "ami":
		imageType = "ami"
	case "qcow2":
		imageType = "qcow2"
	case "aws-bootc":
		imageType = "ami"
	case "guest-image-bootc":
		imageType = "qcow2"
	default:
		err := ErrorInvalidImageType(fmt.Sprintf("Unsupported image type: %v", args.ImageType))
		setResponseError(err, &jobResult)
		return job.Update(&jobResult)
	}

	// Construct the podman command with bootc-image-builder
	cmd := exec.Command(
		"sudo", "podman", "run",
		"--rm",
		"--privileged",
		"--pull=missing",
		"--storage-driver=vfs",
		"--cgroup-manager=cgroupfs",
		"--runtime=runc",
		"--security-opt", "label=type:unconfined_t",
		"-v", "/var/cache/osbuild-composer/output:/output",
		"quay.io/centos-bootc/bootc-image-builder:latest",
		args.ImageRef,
	)

	// Append optional flags
	if imageType != "" {
		cmd.Args = append(cmd.Args, "--type", imageType)
	}
	if args.Arch != "" {
		cmd.Args = append(cmd.Args, "--target-arch", args.Arch)
	}
	if args.Chown != "" {
		cmd.Args = append(cmd.Args, "--chown", args.Chown)
	}
	if !args.TLSVerify {
		cmd.Args = append(cmd.Args, "--tls-verify=false")
	}

	// Log the command being run
	logWithId.Infof("Running command: %s", cmd.String())

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		logWithId.Errorf("Error running bootc-image-builder: %v, Output: %s", err, string(output))
		setResponseError(ErrorCommandExecution(err.Error()), &jobResult)
		return job.Update(&jobResult)
	}

	// Log the output of the command
	logWithId.Infof("bootc-image-builder output: %s", string(output))

	// If the command succeeded, update the job result
	jobResult.JobResult = worker.JobResult{} // No error
	err = job.Update(&jobResult)
	if err != nil {
		setResponseError(ErrorUnknown(fmt.Sprintf("Error reporting job result: %v", err)), &jobResult)
		return job.Update(&jobResult)
	}

	return nil
}

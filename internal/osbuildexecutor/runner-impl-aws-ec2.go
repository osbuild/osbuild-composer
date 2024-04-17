package osbuildexecutor

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

type awsEC2Executor struct {
	iamProfile      string
	keyName         string
	cloudWatchGroup string
}

func prepareSources(manifest []byte, store string, extraEnv []string, result bool, errorWriter io.Writer) error {
	hostExecutor := NewHostExecutor()
	_, err := hostExecutor.RunOSBuild(manifest, store, "", nil, nil, nil, extraEnv, result, errorWriter)
	return err
}

func (ec2e *awsEC2Executor) RunOSBuild(manifest []byte, store, outputDirectory string, exports, exportPaths, checkpoints,
	extraEnv []string, result bool, errorWriter io.Writer) (*osbuild.Result, error) {

	err := prepareSources(manifest, store, extraEnv, result, errorWriter)
	if err != nil {
		return nil, err
	}

	region, err := awscloud.RegionFromInstanceMetadata()
	if err != nil {
		return nil, err
	}

	aws, err := awscloud.NewDefault(region)
	if err != nil {
		return nil, err
	}

	si, err := aws.RunSecureInstance(ec2e.iamProfile, ec2e.keyName, ec2e.cloudWatchGroup)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := aws.TerminateSecureInstance(si)
		if err != nil {
			logrus.Errorf("Error terminating secure instance: %v", err)
		}
	}()

	logrus.Info("Spinning up jobsite manager")
	args := []string{
		"--builder-host",
		*si.Instance.PrivateIpAddress,
		"--store",
		store,
	}

	for _, exp := range exports {
		args = append(args, "--export", exp)
	}
	for _, exp := range exportPaths {
		args = append(args, "--export-file", exp)
	}
	args = append(args, "--output", outputDirectory)

	cmd := exec.Command(
		"/usr/libexec/osbuild-composer/osbuild-jobsite-manager",
		args...,
	)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = bytes.NewReader(manifest)

	err = cmd.Start()
	if err != nil {
		logrus.Errorf("Starting osbuild-jobsite-manager failed: %v", err)
		return nil, err
	}
	err = cmd.Wait()
	if err != nil {
		logrus.Errorf("Waiting for osbuild-jobsite-manager failed: %v", err)
		if e, ok := err.(*exec.ExitError); ok {
			logrus.Errorf("Exit code: %d", e.ExitCode())
		}
		logrus.Errorf("StdErr :%s", stderr.String())
		return nil, err
	}

	var osbuildResult osbuild.Result
	err = json.Unmarshal(stdout.Bytes(), &osbuildResult)
	if err != nil {
		logrus.Errorf("Unable to unmarshal stdout into osbuild result: %v", stdout.String())
		return nil, err
	}
	return &osbuildResult, nil
}

func NewAWSEC2Executor(iamProfile, keyName, cloudWatchGroup string) Executor {
	return &awsEC2Executor{
		iamProfile,
		keyName,
		cloudWatchGroup,
	}
}

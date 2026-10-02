package osbuildexecutor

import (
	"time"

	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

var ExtractOutputArchive = extractOutputArchive
var FetchLog = fetchLog
var FetchOutputArchive = fetchOutputArchive
var HandleBuild = handleBuild
var IsSecureInstanceGone = isSecureInstanceGone
var ValidateOutputArchive = validateOutputArchive
var WaitForSI = waitForSI
var WriteInputArchive = writeInputArchive

type SecureInstanceClient = secureInstanceClient

func NewAWSEC2ExecutorWithSIClient(iamProfile, keyName, hostname, tmpDir string, client SecureInstanceClient, readyTimeout time.Duration) Executor {
	return &awsEC2Executor{
		iamProfile:   iamProfile,
		keyName:      keyName,
		hostname:     hostname,
		tmpDir:       tmpDir,
		siClient:     client,
		readyTimeout: readyTimeout,
	}
}

func MockRunPrepareSources(fn func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error)) (restore func()) {
	saved := runPrepareSources
	runPrepareSources = fn
	return func() {
		runPrepareSources = saved
	}
}

type SIClientFunc struct {
	Run  func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error)
	Term func(si *awscloud.SecureInstance) error
}

func (c SIClientFunc) RunSecureInstance(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
	return c.Run(iamProfile, keyName, hostname)
}

func (c SIClientFunc) TerminateSecureInstance(si *awscloud.SecureInstance) error {
	if c.Term != nil {
		return c.Term(si)
	}
	return nil
}

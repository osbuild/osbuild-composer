package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/sbom"

	"github.com/osbuild/osbuild-composer/internal/upload/oci"
	"github.com/osbuild/osbuild-composer/internal/upload/pulp"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type GCPConfiguration struct {
	Creds  string
	Bucket string
}

type S3Configuration struct {
	Creds               string
	Endpoint            string
	Region              string
	Bucket              string
	CABundle            string
	SkipSSLVerification bool
}

type ContainersConfiguration struct {
	AuthFilePath string
	Domain       string
	PathPrefix   string
	CertPath     string
	TLSVerify    *bool
}

type AzureConfiguration struct {
	Creds         *azure.Credentials
	UploadThreads int
}

type OCIConfiguration struct {
	ClientParams *oci.ClientParams
	Compartment  string
	Bucket       string
	Namespace    string
}

type PulpConfiguration struct {
	CredsFilePath string
	ServerAddress string
}

type ExecutorConfiguration struct {
	Type            string
	IAMProfile      string
	KeyName         string
	CloudWatchGroup string
}

type OSBuildJobImpl struct {
	Store                string
	Output               string
	OSBuildExecutor      ExecutorConfiguration
	KojiServers          map[string]kojiServer
	GCPConfig            GCPConfiguration
	AzureConfig          AzureConfiguration
	OCIConfig            OCIConfiguration
	AWSCreds             string
	AWSBucket            string
	S3Config             S3Configuration
	ContainersConfig     ContainersConfiguration
	PulpConfig           PulpConfiguration
	RepositoryMTLSConfig *RepositoryMTLSConfig
}

// Returns an *awscloud.AWS object with the credentials of the request. If they
// are not accessible, then try to use the one obtained in the worker
// configuration.
func (impl *OSBuildJobImpl) getAWS(region string, accessId string, secret string, token string) (*awscloud.AWS, error) {
	if accessId != "" && secret != "" {
		return awscloud.New(region, accessId, secret, token)
	} else if impl.AWSCreds != "" {
		return awscloud.NewFromFile(impl.AWSCreds, region)
	} else {
		return awscloud.NewDefault(region)
	}
}

func (impl *OSBuildJobImpl) getAWSForS3TargetFromOptions(options *target.AWSS3TargetOptions) (*awscloud.AWS, error) {
	if options.AccessKeyID != "" && options.SecretAccessKey != "" {
		return awscloud.NewForEndpoint(options.Endpoint, options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken, options.CABundle, options.SkipSSLVerification)
	}
	if impl.S3Config.Creds != "" {
		return awscloud.NewForEndpointFromFile(impl.S3Config.Creds, options.Endpoint, options.Region, options.CABundle, options.SkipSSLVerification)
	}
	return nil, fmt.Errorf("no credentials found")
}

func (impl *OSBuildJobImpl) getAWSForS3TargetFromConfig() (*awscloud.AWS, string, error) {
	err := impl.verifyS3TargetConfiguration()
	if err != nil {
		return nil, "", err
	}
	aws, err := awscloud.NewForEndpointFromFile(impl.S3Config.Creds, impl.S3Config.Endpoint, impl.S3Config.Region, impl.S3Config.CABundle, impl.S3Config.SkipSSLVerification)
	return aws, impl.S3Config.Bucket, err
}

func (impl *OSBuildJobImpl) verifyS3TargetConfiguration() error {
	if impl.S3Config.Endpoint == "" {
		return fmt.Errorf("no default endpoint for S3 was set")
	}

	if impl.S3Config.Region == "" {
		return fmt.Errorf("no default region for S3 was set")
	}

	if impl.S3Config.Bucket == "" {
		return fmt.Errorf("no default bucket for S3 was set")
	}

	if impl.S3Config.Creds == "" {
		return fmt.Errorf("no default credentials for S3 was set")
	}

	return nil
}

func (impl *OSBuildJobImpl) getAWSForS3Target(options *target.AWSS3TargetOptions) (*awscloud.AWS, string, error) {
	var aws *awscloud.AWS = nil
	var err error

	bucket := options.Bucket

	// Endpoint == "" && Region != "" => AWS (Weldr and Composer)
	if options.Endpoint == "" && options.Region != "" {
		aws, err = impl.getAWS(options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken)
		if bucket == "" {
			bucket = impl.AWSBucket
			if bucket == "" {
				err = fmt.Errorf("No AWS bucket provided")
			}
		}
	} else if options.Endpoint != "" && options.Region != "" { // Endpoint != "" && Region != "" => Generic S3 Weldr API
		aws, err = impl.getAWSForS3TargetFromOptions(options)
	} else if options.Endpoint == "" && options.Region == "" { // Endpoint == "" && Region == "" => Generic S3 Composer API
		aws, bucket, err = impl.getAWSForS3TargetFromConfig()
	} else {
		err = fmt.Errorf("s3 server configuration is incomplete")
	}

	return aws, bucket, err
}

// getGCP returns an *gcp.GCP object using credentials based on the following
// predefined preference:
//
//  1. If the provided `credentials` parameter is not `nil`, it is used to
//     authenticate with GCP.
//
//  2. If a path to GCP credentials file was provided in the worker's
//     configuration, it is used to authenticate with GCP.
//
//  3. Use Application Default Credentials from the Google library, which tries
//     to automatically find a way to authenticate using the following options:
//
//     3a. If `GOOGLE_APPLICATION_CREDENTIALS` environment variable is set, it
//     tries to load and use credentials form the file pointed to by the
//     variable.
//
//     3b. It tries to authenticate using the service account attached to the
//     resource which is running the code (e.g. Google Compute Engine VM).
func (impl *OSBuildJobImpl) getGCP(credentials []byte) (*gcp.GCP, error) {
	if credentials != nil {
		logrus.Info("[GCP] 🔑 using credentials provided with the job request")
		return gcp.New(credentials)
	} else if impl.GCPConfig.Creds != "" {
		logrus.Info("[GCP] 🔑 using credentials from the worker configuration")
		return gcp.NewFromFile(impl.GCPConfig.Creds)
	} else {
		logrus.Info("[GCP] 🔑 using Application Default Credentials via Google library")
		return gcp.New(nil)
	}
}

// Takes the worker config as a base and overwrites it with both t1 and t2's options
func (impl *OSBuildJobImpl) getOCI(tcp oci.ClientParams) (oci.Client, error) {
	var cp oci.ClientParams
	if impl.OCIConfig.ClientParams != nil {
		cp = *impl.OCIConfig.ClientParams
	}
	if tcp.User != "" {
		cp.User = tcp.User
	}
	if tcp.Region != "" {
		cp.Region = tcp.Region
	}
	if tcp.Tenancy != "" {
		cp.Tenancy = tcp.Tenancy
	}
	if tcp.PrivateKey != "" {
		cp.PrivateKey = tcp.PrivateKey
	}
	if tcp.Fingerprint != "" {
		cp.Fingerprint = tcp.Fingerprint
	}
	return oci.NewClient(&cp)
}

func validateResult(result *worker.OSBuildJobResult, jobID string) {
	logWithId := logrus.WithField("jobId", jobID)
	if result.JobError != nil {
		logWithId.Errorf("osbuild job failed: %s", result.JobError.Reason)
		if result.JobError.Details != nil {
			logWithId.Errorf("failure details : %v", result.JobError.Details)
		}
		return
	}
	// if the job failed, but the JobError is
	// nil, we still need to handle this as an error
	if result.OSBuildOutput == nil || !result.OSBuildOutput.Success {
		reason := "osbuild job was unsuccessful"
		logWithId.Errorf("osbuild job failed: %s", reason)
		result.JobError = clienterrors.New(clienterrors.ErrorBuildJob, reason, nil)
		return
	} else {
		logWithId.Infof("osbuild job succeeded")
	}
	result.Success = true
}

func uploadToS3(a *awscloud.AWS, outputDirectory, exportPath, bucket, key, filename string, public bool) (string, *clienterrors.Error) {
	imagePath := path.Join(outputDirectory, exportPath, filename)

	if key == "" {
		key = uuid.New().String()
	}
	key += "-" + filename

	result, err := a.Upload(imagePath, bucket, key)
	if err != nil {
		return "", clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)

	}

	if public {
		err := a.MarkS3ObjectAsPublic(bucket, key)
		if err != nil {
			return "", clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
		}

		return result.Location, nil
	}

	url, err := a.S3ObjectPresignedURL(bucket, key)
	if err != nil {
		return "", clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
	}

	return url, nil
}

func (impl *OSBuildJobImpl) getContainerClient(destination string, targetOptions *target.ContainerTargetOptions) (*container.Client, error) {
	destination, appliedDefaults := container.ApplyDefaultDomainPath(destination, impl.ContainersConfig.Domain, impl.ContainersConfig.PathPrefix)
	client, err := container.NewClient(destination)
	if err != nil {
		return nil, err
	}

	if impl.ContainersConfig.AuthFilePath != "" {
		client.SetAuthFilePath(impl.ContainersConfig.AuthFilePath)
	}

	if appliedDefaults {

		if impl.ContainersConfig.CertPath != "" {
			client.SetDockerCertPath(impl.ContainersConfig.CertPath)
		}
		client.SetTLSVerify(impl.ContainersConfig.TLSVerify)
	} else {
		if targetOptions.Username != "" || targetOptions.Password != "" {
			client.SetCredentials(targetOptions.Username, targetOptions.Password)
		}
		client.SetTLSVerify(targetOptions.TlsVerify)
	}

	return client, nil
}

// Read server configuration and credentials from the target options and fall
// back to worker config if they are not set (targetOptions take precedent).
// Mixing sources is allowed. For example, the server address can be configured
// in the worker config while the targetOptions provide the credentials (or
// vice versa).
func (impl *OSBuildJobImpl) getPulpClient(targetOptions *target.PulpOSTreeTargetOptions) (*pulp.Client, error) {

	var creds *pulp.Credentials
	// Credentials are considered together. In other words, the username can't
	// come from a different config source than the password.
	if targetOptions.Username != "" && targetOptions.Password != "" {
		creds = &pulp.Credentials{
			Username: targetOptions.Username,
			Password: targetOptions.Password,
		}
	}
	address := targetOptions.ServerAddress
	if address == "" {
		// fall back to worker configuration for server address
		address = impl.PulpConfig.ServerAddress
	}
	if address == "" {
		return nil, fmt.Errorf("pulp server address not set")
	}

	if creds != nil {
		return pulp.NewClient(address, creds), nil
	}

	// read from worker configuration
	if impl.PulpConfig.CredsFilePath == "" {
		return nil, fmt.Errorf("pulp credentials not set")
	}

	// use creds file loader helper
	return pulp.NewClientFromFile(address, impl.PulpConfig.CredsFilePath)
}

func makeJobErrorFromOsbuildOutput(osbuildOutput *osbuild.Result) *clienterrors.Error {
	var osbErrors []string
	if osbuildOutput.Error != nil {
		osbErrors = append(osbErrors, fmt.Sprintf("osbuild error: %s", string(osbuildOutput.Error)))
	}
	if osbuildOutput.Errors != nil {
		for _, err := range osbuildOutput.Errors {
			osbErrors = append(osbErrors, fmt.Sprintf("manifest validation error: %v", err))
		}
	}
	var failedStage string
	for _, pipelineLog := range osbuildOutput.Log {
		for _, stageResult := range pipelineLog {
			if !stageResult.Success {
				failedStage = stageResult.Type
				break
			}
		}
	}

	reason := "osbuild build failed"
	if failedStage != "" {
		reason += fmt.Sprintf(" in stage: %q", failedStage)
	}
	return clienterrors.New(clienterrors.ErrorBuildJob, reason, osbErrors)
}

func (impl *OSBuildJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id().String())
	// Initialize variable needed for reporting back to osbuild-composer.
	var osbuildJobResult *worker.OSBuildJobResult = &worker.OSBuildJobResult{
		Success: false,
		OSBuildOutput: &osbuild.Result{
			Success: false,
		},
		UploadStatus: "failure",
		Arch:         arch.Current().String(),
	}

	hostOS, err := distro.GetHostDistroName()
	if err != nil {
		logWithId.Warnf("Failed to get host distro name: %v", err)
		hostOS = "linux"
	}
	osbuildJobResult.HostOS = hostOS

	// In all cases it is necessary to report result back to osbuild-composer worker API.
	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic: %v", r)
			logWithId.Errorf("%s", debug.Stack())

			osbuildJobResult.JobError = clienterrors.New(
				clienterrors.ErrorJobPanicked,
				fmt.Sprintf("job panicked:\n%v\n\noriginal error:\n%v", r, osbuildJobResult.JobError),
				nil,
			)
		}
		validateResult(osbuildJobResult, job.Id().String())

		err := job.Update(osbuildJobResult)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	outputDirectory, err := os.MkdirTemp(impl.Output, job.Id().String()+"-*")
	if err != nil {
		return fmt.Errorf("error creating temporary output directory: %v", err)
	}
	defer func() {
		err = os.RemoveAll(outputDirectory)
		if err != nil {
			logWithId.Errorf("Error removing temporary output directory (%s): %v", outputDirectory, err)
		}
	}()

	osbuildVersion, err := osbuild.OSBuildVersion()
	if err != nil {
		osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorBuildJob, "Error getting osbuild binary version", err.Error())
		return err
	}
	osbuildJobResult.OSBuildVersion = osbuildVersion

	// Read the job specification
	var jobArgs worker.OSBuildJob
	err = job.Args(&jobArgs)
	if err != nil {
		return err
	}

	// In case the manifest is empty, try to get it from dynamic args
	var manifestInfo *worker.ManifestInfo
	if len(jobArgs.Manifest) == 0 {
		if job.NDynamicArgs() > 0 {
			var manifestJR worker.ManifestJobByIDResult
			if job.NDynamicArgs() == 1 {
				// Classic case of a compose request with the ManifestJobByID job as the single dependency
				err = job.DynamicArgs(0, &manifestJR)
			} else if job.NDynamicArgs() > 1 && jobArgs.ManifestDynArgsIdx != nil {
				// Case when the job has multiple dependencies, but the manifest is not part of the static job arguments,
				// but rather in the dynamic arguments (e.g. from ManifestJobByID job).
				if *jobArgs.ManifestDynArgsIdx > job.NDynamicArgs()-1 {
					panic("ManifestDynArgsIdx is out of range of the number of dynamic job arguments")
				}
				err = job.DynamicArgs(*jobArgs.ManifestDynArgsIdx, &manifestJR)
			}
			if err != nil {
				osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args", nil)
				return err
			}

			// skip the job if the manifest generation failed
			if manifestJR.JobError != nil {
				osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorManifestDependency, "Manifest dependency failed", nil)
				return nil
			}
			jobArgs.Manifest = manifestJR.Manifest
			manifestInfo = &manifestJR.ManifestInfo
		}

		if len(jobArgs.Manifest) == 0 {
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorEmptyManifest, "Job has no manifest", nil)
			return nil
		}
	}

	// Explicitly check that none of the job dependencies failed.
	// This covers mainly the case when there are more than one job dependencies.
	for idx := 0; idx < job.NDynamicArgs(); idx++ {
		var jobResult worker.JobResult
		err = job.DynamicArgs(idx, &jobResult)
		if err != nil {
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args", nil)
			return err
		}

		if jobResult.JobError != nil {
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorJobDependency, "Job dependency failed", nil)
			return nil
		}
	}

	// copy pipeline info to the result
	osbuildJobResult.PipelineNames = jobArgs.PipelineNames

	// copy the image boot mode to the result
	osbuildJobResult.ImageBootMode = jobArgs.ImageBootMode

	// get exports for all job's targets
	exports := jobArgs.OsbuildExports()
	if len(exports) == 0 {
		osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "no osbuild export specified for the job", nil)
		return nil
	}

	var extraEnv []string
	if impl.ContainersConfig.AuthFilePath != "" {
		extraEnv = []string{
			fmt.Sprintf("REGISTRY_AUTH_FILE=%s", impl.ContainersConfig.AuthFilePath),
		}
	}

	// Both curl and ostree input share the same MTLS config
	if impl.RepositoryMTLSConfig != nil {
		// Setting a CA cert with hosted Pulp with break the build since Pulp redirects HTTPS requests to AWS S3 which has
		// a different CA which is part of OS cert bundle. Both curl and ostree commands only support either explicit CA file
		// or OS cert bundle, but not both. To verify hosted Pulp CA, enroll its CA into the OS cert bundle instead.
		if impl.RepositoryMTLSConfig.CA != "" {
			extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CA_CERT=%s", impl.RepositoryMTLSConfig.CA))
			extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_OSTREE_SSL_CA_CERT=%s", impl.RepositoryMTLSConfig.CA))
		}
		extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CLIENT_KEY=%s", impl.RepositoryMTLSConfig.MTLSClientKey))
		extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CLIENT_CERT=%s", impl.RepositoryMTLSConfig.MTLSClientCert))
		extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_OSTREE_SSL_CLIENT_KEY=%s", impl.RepositoryMTLSConfig.MTLSClientKey))
		extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_OSTREE_SSL_CLIENT_CERT=%s", impl.RepositoryMTLSConfig.MTLSClientCert))
		if impl.RepositoryMTLSConfig.Proxy != nil {
			extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_CURL_PROXY=%s", impl.RepositoryMTLSConfig.Proxy.String()))
			extraEnv = append(extraEnv, fmt.Sprintf("OSBUILD_SOURCES_OSTREE_PROXY=%s", impl.RepositoryMTLSConfig.Proxy.String()))
		}
	}

	// Run osbuild and handle two kinds of errors
	var executor osbuildexecutor.Executor
	switch impl.OSBuildExecutor.Type {
	case "host":
		executor = osbuildexecutor.NewHostExecutor()
	case "aws.ec2":
		err = os.MkdirAll("/var/tmp/osbuild-composer", 0755)
		if err != nil {
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorInvalidConfig, "Unable to create /var/tmp/osbuild-composer needed to aws.ec2 executor", nil)
			return err
		}
		tmpDir, err := os.MkdirTemp("/var/tmp/osbuild-composer", "")
		if err != nil {
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorInvalidConfig, "Unable to create /var/tmp/osbuild-composer needed to aws.ec2 executor", nil)
			return err
		}
		defer os.RemoveAll(tmpDir)
		executor = osbuildexecutor.NewAWSEC2Executor(impl.OSBuildExecutor.IAMProfile, impl.OSBuildExecutor.KeyName, impl.OSBuildExecutor.CloudWatchGroup, job.Id().String(), tmpDir)
	default:
		osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorInvalidConfig, "No osbuild executor defined", nil)
		return err
	}

	exportPaths := []string{}
	for _, jobTarget := range jobArgs.Targets {
		exportPaths = append(exportPaths, path.Join(jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename))
	}

	logWithId.Infof("Extra env: %q", extraEnv)
	opts := &osbuildexecutor.OsbuildOpts{
		StoreDir:    impl.Store,
		OutputDir:   outputDirectory,
		Exports:     exports,
		ExportPaths: exportPaths,
		ExtraEnv:    extraEnv,
		Result:      true,
	}
	osbuildJobResult.OSBuildOutput, err = executor.RunOSBuild(jobArgs.Manifest, opts, os.Stderr)
	// First handle the case when "running" osbuild failed
	if err != nil {
		osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorBuildJob, "osbuild build failed", err.Error())
		return err
	}

	// Include pipeline stages output inside the worker's logs.
	// Order pipelines based on PipelineNames from job
	for _, pipelineName := range osbuildJobResult.PipelineNames.All() {
		pipelineLog, hasLog := osbuildJobResult.OSBuildOutput.Log[pipelineName]
		if !hasLog {
			// no pipeline output
			continue
		}
		logWithId.Infof("%s pipeline results:\n", pipelineName)
		for _, stageResult := range pipelineLog {
			if stageResult.Success {
				logWithId.Infof("  %s success", stageResult.Type)
			} else {
				logWithId.Infof("  %s failure:", stageResult.Type)
				stageOutput := strings.Split(stageResult.Output, "\n")
				for _, line := range stageOutput {
					logWithId.Infof("    %s", line)
				}
			}
		}
	}

	// Second handle the case when the build failed, but osbuild finished successfully
	if !osbuildJobResult.OSBuildOutput.Success {
		osbuildJobResult.JobError = makeJobErrorFromOsbuildOutput(osbuildJobResult.OSBuildOutput)
		return nil
	}

	for _, jobTarget := range jobArgs.Targets {
		var targetResult *target.TargetResult
		artifact := jobTarget.OsbuildArtifact
		switch targetOptions := jobTarget.Options.(type) {
		case *target.WorkerServerTargetOptions:
			targetResult = target.NewWorkerServerTargetResult(&target.WorkerServerTargetResultOptions{
				ArtifactRelPath: path.Join(jobTarget.OsbuildArtifact.ExportFilename),
			}, &artifact)
			var f *os.File
			imagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)
			f, err = os.Open(imagePath)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, err.Error(), nil)
				break
			}
			defer f.Close()
			err = job.UploadArtifact(jobTarget.ImageName, f)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

		case *target.VMWareTargetOptions:
			targetResult = target.NewVMWareTargetResult(&artifact)
			credentials := vmware.Credentials{
				Username:   targetOptions.Username,
				Password:   targetOptions.Password,
				Host:       targetOptions.Host,
				Cluster:    targetOptions.Cluster,
				Datacenter: targetOptions.Datacenter,
				Datastore:  targetOptions.Datastore,
				Folder:     targetOptions.Folder,
			}

			tempDirectory, err := os.MkdirTemp(impl.Output, job.Id().String()+"-vmware-*")
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			defer func() {
				err := os.RemoveAll(tempDirectory)
				if err != nil {
					logWithId.Errorf("Error removing temporary directory for vmware symlink(%s): %v", tempDirectory, err)
				}
			}()

			exportedImagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)

			if strings.HasSuffix(exportedImagePath, ".vmdk") {
				// create a symlink so that uploaded image has the name specified by user
				imageName := jobTarget.ImageName + ".vmdk"
				imagePath := path.Join(tempDirectory, imageName)

				err = os.Symlink(exportedImagePath, imagePath)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
					break
				}

				err = vmware.ImportVmdk(credentials, imagePath)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
					break
				}
			} else if strings.HasSuffix(exportedImagePath, ".ova") {
				err = vmware.ImportOva(credentials, exportedImagePath, jobTarget.ImageName)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
					break
				}
			} else {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, "No vmdk or ova provided", nil)
				break
			}

		case *target.AWSTargetOptions:
			targetResult = target.NewAWSTargetResult(nil, &artifact)
			a, err := impl.getAWS(targetOptions.Region, targetOptions.AccessKeyID, targetOptions.SecretAccessKey, targetOptions.SessionToken)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			if targetOptions.Key == "" {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "No AWS object key provided", nil)
				break
			}

			bucket := targetOptions.Bucket
			if bucket == "" {
				bucket = impl.AWSBucket
				if bucket == "" {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "No AWS bucket provided", nil)
					break
				}
			}

			// TODO: Remove this once multiple exports will be supported and used by image definitions
			// RHUI images tend to be produced as archives in Brew to save disk space,
			// however they can't be imported to the cloud provider as an archive.
			// Workaround this situation for Koji composes by checking if the image file
			// is an archive and if it is, extract it before uploading to the cloud.
			imagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)
			if strings.HasSuffix(imagePath, ".xz") {
				imagePath, err = extractXzArchive(imagePath)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorTargetError, "Failed to extract compressed image", err.Error())
					break
				}
			}

			_, err = a.Upload(imagePath, bucket, targetOptions.Key)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

			ami, err := a.Register(jobTarget.ImageName, bucket, targetOptions.Key, targetOptions.ShareWithAccounts, arch.Current().String(), targetOptions.BootMode)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorImportingImage, err.Error(), nil)
				break
			}

			if ami == nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorImportingImage, "No ami returned", nil)
				break
			}
			targetResult.Options = &target.AWSTargetResultOptions{
				Ami:    *ami,
				Region: targetOptions.Region,
			}

		case *target.AWSS3TargetOptions:
			targetResult = target.NewAWSS3TargetResult(nil, &artifact)
			a, bucket, err := impl.getAWSForS3Target(targetOptions)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			if targetOptions.Key == "" {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "No AWS object key provided", nil)
				break
			}

			url, targetError := uploadToS3(a, outputDirectory, jobTarget.OsbuildArtifact.ExportName, bucket, targetOptions.Key, jobTarget.OsbuildArtifact.ExportFilename, targetOptions.Public)
			if targetError != nil {
				targetResult.TargetError = targetError
				break
			}
			targetResult.Options = &target.AWSS3TargetResultOptions{URL: url}

		case *target.AzureTargetOptions:
			targetResult = target.NewAzureTargetResult(&artifact)
			azureStorageClient, err := azure.NewStorageClient(targetOptions.StorageAccount, targetOptions.StorageAccessKey)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			// Azure cannot create an image from a blob without .vhd extension
			blobName := azure.EnsureVHDExtension(jobTarget.ImageName)
			metadata := azure.BlobMetadata{
				StorageAccount: targetOptions.StorageAccount,
				ContainerName:  targetOptions.Container,
				BlobName:       blobName,
			}

			const azureMaxUploadGoroutines = 4
			err = azureStorageClient.UploadPageBlob(
				metadata,
				path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename),
				azureMaxUploadGoroutines,
			)

			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

		case *target.GCPTargetOptions:
			targetResult = target.NewGCPTargetResult(nil, &artifact)
			ctx := context.Background()

			g, err := impl.getGCP(targetOptions.Credentials)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			if targetOptions.Object == "" {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "No GCP object key provided", nil)
				break
			}

			bucket := targetOptions.Bucket
			if bucket == "" {
				bucket = impl.GCPConfig.Bucket
				if bucket == "" {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "No GCP bucket provided", nil)
					break
				}
			}

			logWithId.Infof("[GCP] 🚀 Uploading image to: %s/%s", bucket, targetOptions.Object)
			_, err = g.StorageObjectUpload(ctx, path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename),
				bucket, targetOptions.Object, map[string]string{gcp.MetadataKeyImageName: jobTarget.ImageName})
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

			guestOSFeatures := targetOptions.GuestOsFeatures
			// TODO: Remove this after "some time"
			// This is just a backward compatibility for the old composer versions,
			// which did not set the guest OS features in the target options.
			if len(guestOSFeatures) == 0 {
				guestOSFeatures = gcp.GuestOsFeaturesByDistro(targetOptions.Os)
			}

			logWithId.Infof("[GCP] 📥 Importing image into Compute Engine as '%s'", jobTarget.ImageName)

			_, importErr := g.ComputeImageInsert(ctx, bucket, targetOptions.Object, jobTarget.ImageName, []string{targetOptions.Region}, guestOSFeatures)
			if importErr == nil {
				logWithId.Infof("[GCP] 🎉 Image import finished successfully")
			}

			// Cleanup storage before checking for errors
			logWithId.Infof("[GCP] 🧹 Deleting uploaded image file: %s/%s", bucket, targetOptions.Object)
			if err = g.StorageObjectDelete(ctx, bucket, targetOptions.Object); err != nil {
				logWithId.Errorf("[GCP] Encountered error while deleting object: %v", err)
			}

			// check error from ComputeImageInsert()
			if importErr != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorImportingImage, importErr.Error(), nil)
				break
			}
			logWithId.Infof("[GCP] 💿 Image URL: %s", g.ComputeImageURL(jobTarget.ImageName))

			if len(targetOptions.ShareWithAccounts) > 0 {
				logWithId.Infof("[GCP] 🔗 Sharing the image with: %+v", targetOptions.ShareWithAccounts)
				err = g.ComputeImageShare(ctx, jobTarget.ImageName, targetOptions.ShareWithAccounts)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorSharingTarget, err.Error(), nil)
					break
				}
			}
			targetResult.Options = &target.GCPTargetResultOptions{
				ImageName: jobTarget.ImageName,
				ProjectID: g.GetProjectID(),
			}

		case *target.AzureImageTargetOptions:
			targetResult = target.NewAzureImageTargetResult(nil, &artifact)
			ctx := context.Background()

			if impl.AzureConfig.Creds == nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorSharingTarget, "osbuild job has org.osbuild.azure.image target but this worker doesn't have azure credentials", nil)
				break
			}

			c, err := azure.NewClient(*impl.AzureConfig.Creds, targetOptions.TenantID, targetOptions.SubscriptionID)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, err.Error(), nil)
				break
			}
			logWithId.Info("[Azure] 🔑 Logged in Azure")

			location := targetOptions.Location
			if location == "" {
				location, err = c.GetResourceGroupLocation(ctx, targetOptions.ResourceGroup)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("retrieving resource group location failed: %v", err), nil)
					break
				}
			}

			storageAccountTag := azure.Tag{
				Name:  "imageBuilderStorageAccount",
				Value: fmt.Sprintf("location=%s", location),
			}

			storageAccount, err := c.GetResourceNameByTag(
				ctx,
				targetOptions.ResourceGroup,
				storageAccountTag,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("searching for a storage account failed: %v", err), nil)
				break
			}

			if storageAccount == "" {
				logWithId.Info("[Azure] 📦 Creating a new storage account")
				const storageAccountPrefix = "ib"
				storageAccount = azure.RandomStorageAccountName(storageAccountPrefix)

				err := c.CreateStorageAccount(
					ctx,
					targetOptions.ResourceGroup,
					storageAccount,
					location,
					storageAccountTag,
				)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating a new storage account failed: %v", err), nil)
					break
				}
			}

			logWithId.Info("[Azure] 🔑📦 Retrieving a storage account key")
			storageAccessKey, err := c.GetStorageAccountKey(
				ctx,
				targetOptions.ResourceGroup,
				storageAccount,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("retrieving the storage account key failed: %v", err), nil)
				break
			}

			azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating the storage client failed: %v", err), nil)
				break
			}

			storageContainer := "imagebuilder"

			logWithId.Info("[Azure] 📦 Ensuring that we have a storage container")
			err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("cannot create a storage container: %v", err), nil)
				break
			}

			// Azure cannot create an image from a blob without .vhd extension
			blobName := azure.EnsureVHDExtension(jobTarget.ImageName)

			// TODO: Remove this once multiple exports will be supported and used by image definitions
			// RHUI images tend to be produced as archives in Brew to save disk space,
			// however they can't be imported to the cloud provider as an archive.
			// Workaround this situation for Koji composes by checking if the image file
			// is an archive and if it is, extract it before uploading to the cloud.
			imagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)
			if strings.HasSuffix(imagePath, ".xz") {
				imagePath, err = extractXzArchive(imagePath)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorTargetError, "Failed to extract compressed image", err.Error())
					break
				}
			}

			logWithId.Info("[Azure] ⬆ Uploading the image")
			err = azureStorageClient.UploadPageBlob(
				azure.BlobMetadata{
					StorageAccount: storageAccount,
					ContainerName:  storageContainer,
					BlobName:       blobName,
				},
				imagePath,
				impl.AzureConfig.UploadThreads,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, fmt.Sprintf("uploading the image failed: %v", err), nil)
				break
			}

			logWithId.Info("[Azure] 📝 Registering the image")
			err = c.RegisterImage(
				ctx,
				targetOptions.SubscriptionID,
				targetOptions.ResourceGroup,
				storageAccount,
				storageContainer,
				blobName,
				jobTarget.ImageName,
				location,
				targetOptions.HyperVGeneration,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorImportingImage, fmt.Sprintf("registering the image failed: %v", err), nil)
				break
			}
			logWithId.Info("[Azure] 🎉 Image uploaded and registered!")
			targetResult.Options = &target.AzureImageTargetResultOptions{
				ImageName: jobTarget.ImageName,
			}

		case *target.KojiTargetOptions:
			targetResult = target.NewKojiTargetResult(nil, &artifact)
			kojiServerURL, err := url.Parse(targetOptions.Server)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("failed to parse Koji server URL: %v", err), nil)
				break
			}

			kojiServer, exists := impl.KojiServers[kojiServerURL.Hostname()]
			if !exists {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("Koji server has not been configured: %s", kojiServerURL.Hostname()), nil)
				break
			}

			kojiTransport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor, NewRHLeveledLogger(nil))

			kojiAPI, err := koji.NewFromGSSAPI(targetOptions.Server, &kojiServer.creds, kojiTransport, NewRHLeveledLogger(nil))
			if err != nil {
				logWithId.Warnf("[Koji] 🔑 login failed: %v", err) // DON'T EDIT: Used for Splunk dashboard
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("failed to authenticate with Koji server %q: %v", kojiServerURL.Hostname(), err), nil)
				break
			}
			logWithId.Infof("[Koji] 🔑 Authenticated with %q", kojiServerURL.Hostname())
			defer func() {
				err := kojiAPI.Logout()
				if err != nil {
					logWithId.Warnf("[Koji] logout failed: %v", err)
				}
			}()

			file, err := os.Open(path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename))
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorKojiBuild, fmt.Sprintf("failed to open the image for reading: %v", err), nil)
				break
			}
			defer file.Close()

			logWithId.Info("[Koji] ⬆ Uploading the image")
			imageHash, imageSize, err := kojiAPI.Upload(file, targetOptions.UploadDirectory, jobTarget.ImageName)
			if err != nil {
				logWithId.Warnf("[Koji] ⬆ upload failed: %v", err) // DON'T EDIT: used for Splunk dashboard
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}
			logWithId.Info("[Koji] 🎉 Image successfully uploaded")

			var manifest bytes.Buffer
			err = json.Indent(&manifest, jobArgs.Manifest, "", "  ")
			if err != nil {
				logWithId.Warnf("[Koji] Indenting osbuild manifest failed: %v", err)
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorKojiBuild, err.Error(), nil)
				break
			}
			logWithId.Info("[Koji] ⬆ Uploading the osbuild manifest")
			manifestFilename := jobTarget.ImageName + ".manifest.json"
			manifestHash, manifestSize, err := kojiAPI.Upload(&manifest, targetOptions.UploadDirectory, manifestFilename)
			if err != nil {
				logWithId.Warnf("[Koji] ⬆ upload failed: %v", err)
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}
			logWithId.Info("[Koji] 🎉 Manifest successfully uploaded")

			var osbuildLog bytes.Buffer
			err = osbuildJobResult.OSBuildOutput.Write(&osbuildLog)
			if err != nil {
				logWithId.Warnf("[Koji] Converting osbuild log to text failed: %v", err)
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorKojiBuild, err.Error(), nil)
				break
			}
			logWithId.Info("[Koji] ⬆ Uploading the osbuild output log")
			osbuildOutputFilename := jobTarget.ImageName + ".osbuild.log"
			osbuildOutputHash, osbuildOutputSize, err := kojiAPI.Upload(&osbuildLog, targetOptions.UploadDirectory, osbuildOutputFilename)
			if err != nil {
				logWithId.Warnf("[Koji] ⬆ upload failed: %v", err)
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}
			logWithId.Info("[Koji] 🎉 osbuild output log successfully uploaded")

			// Attach the manifest info to the koji target result, so that it
			// it can be imported to the Koji build by the koji-finalize job.
			var kojiManifestInfo *target.ManifestInfo
			if manifestInfo != nil {
				kojiManifestInfo = &target.ManifestInfo{
					OSBuildComposerVersion: manifestInfo.OSBuildComposerVersion,
				}
				for _, composerDep := range manifestInfo.OSBuildComposerDeps {
					dep := &target.OSBuildComposerDepModule{
						Path:    composerDep.Path,
						Version: composerDep.Version,
					}
					if composerDep.Replace != nil {
						dep.Replace = &target.OSBuildComposerDepModule{
							Path:    composerDep.Replace.Path,
							Version: composerDep.Replace.Version,
						}
					}
					kojiManifestInfo.OSBuildComposerDeps = append(kojiManifestInfo.OSBuildComposerDeps, dep)
				}
			}

			var sbomDocsInfo []target.KojiOutputInfo
			if jobArgs.DepsolveDynArgsIdx != nil {
				depsolveJRIdx := *jobArgs.DepsolveDynArgsIdx
				if depsolveJRIdx > job.NDynamicArgs()-1 {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "DepsolveDynArgsIdx is out of range of the number of dynamic job arguments", nil)
					break
				}

				var depsolveJR worker.DepsolveJobResult
				err = job.DynamicArgs(depsolveJRIdx, &depsolveJR)
				if err != nil {
					targetResult.TargetError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "Error parsing DepsolveJobResult from dynamic args", nil)
					break
				}

				logWithId.Info("[Koji] ⬆ Uploading SBOM documents")
				for pipelineName, sbomDoc := range depsolveJR.SbomDocs {
					var pipelinePurpose string
					if slices.Contains(jobArgs.PipelineNames.Payload, pipelineName) {
						pipelinePurpose = "image"
					}
					if slices.Contains(jobArgs.PipelineNames.Build, pipelineName) {
						pipelinePurpose = "buildroot"
					}

					var sbomDocExtension string
					if sbomDoc.DocType == sbom.StandardTypeSpdx {
						sbomDocExtension = "spdx.json"
					} else {
						targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, fmt.Sprintf("Unsupported SBOM document type: %s", sbomDoc.DocType), nil)
						break
					}

					reader := bytes.NewReader(sbomDoc.Document)
					sbomDocOutputFilename := fmt.Sprintf("%s.%s-%s.%s", jobTarget.ImageName, pipelinePurpose, pipelineName, sbomDocExtension)
					sbomDocOutputHash, sbomDocOutputSize, err := kojiAPI.Upload(reader, targetOptions.UploadDirectory, sbomDocOutputFilename)
					if err != nil {
						logWithId.Warnf("[Koji] ⬆ upload failed: %v", err)
						targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
						break
					}
					sbomDocsInfo = append(sbomDocsInfo, target.KojiOutputInfo{
						Filename:     sbomDocOutputFilename,
						ChecksumType: target.ChecksumTypeMD5,
						Checksum:     sbomDocOutputHash,
						Size:         sbomDocOutputSize,
					})
				}
				logWithId.Info("[Koji] 🎉 SBOM documents successfully uploaded")
			}

			targetResult.Options = &target.KojiTargetResultOptions{
				Image: &target.KojiOutputInfo{
					Filename:     jobTarget.ImageName,
					ChecksumType: target.ChecksumTypeMD5,
					Checksum:     imageHash,
					Size:         imageSize,
				},
				OSBuildManifest: &target.KojiOutputInfo{
					Filename:     manifestFilename,
					ChecksumType: target.ChecksumTypeMD5,
					Checksum:     manifestHash,
					Size:         manifestSize,
				},
				Log: &target.KojiOutputInfo{
					Filename:     osbuildOutputFilename,
					ChecksumType: target.ChecksumTypeMD5,
					Checksum:     osbuildOutputHash,
					Size:         osbuildOutputSize,
				},
				OSBuildManifestInfo: kojiManifestInfo,
				SbomDocs:            sbomDocsInfo,
			}

		case *target.OCITargetOptions:
			targetResult = target.NewOCITargetResult(nil, &artifact)
			// create an ociClient uploader with a valid storage client
			var ociClient oci.Client
			ociClient, err = impl.getOCI(oci.ClientParams{
				User:        targetOptions.User,
				Region:      targetOptions.Region,
				Tenancy:     targetOptions.Tenancy,
				Fingerprint: targetOptions.Fingerprint,
				PrivateKey:  targetOptions.PrivateKey,
			})
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}
			logWithId.Info("[OCI] 🔑 Logged in OCI")
			logWithId.Info("[OCI] ⬆ Uploading the image")
			file, err := os.Open(path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename))
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}
			defer file.Close()
			i, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			bucket := impl.OCIConfig.Bucket
			if targetOptions.Bucket != "" {
				bucket = targetOptions.Bucket
			}
			namespace := impl.OCIConfig.Namespace
			if targetOptions.Namespace != "" {
				namespace = targetOptions.Namespace
			}
			err = ociClient.Upload(
				fmt.Sprintf("osbuild-upload-%d", i),
				bucket,
				namespace,
				file,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

			compartment := impl.OCIConfig.Compartment
			if targetOptions.Compartment != "" {
				compartment = targetOptions.Compartment
			}
			imageID, err := ociClient.CreateImage(
				fmt.Sprintf("osbuild-upload-%d", i),
				bucket,
				namespace,
				compartment,
				jobTarget.ImageName,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

			logWithId.Info("[OCI] 🎉 Image uploaded and registered!")
			targetResult.Options = &target.OCITargetResultOptions{ImageID: imageID}
		case *target.OCIObjectStorageTargetOptions:
			targetResult = target.NewOCIObjectStorageTargetResult(nil, &artifact)
			// create an ociClient uploader with a valid storage client
			ociClient, err := impl.getOCI(oci.ClientParams{
				User:        targetOptions.User,
				Region:      targetOptions.Region,
				Tenancy:     targetOptions.Tenancy,
				Fingerprint: targetOptions.Fingerprint,
				PrivateKey:  targetOptions.PrivateKey,
			})
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}
			logWithId.Info("[OCI] 🔑 Logged in OCI")
			logWithId.Info("[OCI] ⬆ Uploading the image")
			file, err := os.Open(path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename))
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}
			defer file.Close()
			i, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			bucket := impl.OCIConfig.Bucket
			if targetOptions.Bucket != "" {
				bucket = targetOptions.Bucket
			}
			namespace := impl.OCIConfig.Namespace
			if targetOptions.Namespace != "" {
				namespace = targetOptions.Namespace
			}
			err = ociClient.Upload(
				fmt.Sprintf("osbuild-upload-%d", i),
				bucket,
				namespace,
				file,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}

			uri, err := ociClient.PreAuthenticatedRequest(fmt.Sprintf("osbuild-upload-%d", i), bucket, namespace)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorGeneratingSignedURL, err.Error(), nil)
				break
			}
			logWithId.Info("[OCI] 🎉 Image uploaded and pre-authenticated request generated!")
			targetResult.Options = &target.OCIObjectStorageTargetResultOptions{URL: uri}
		case *target.ContainerTargetOptions:
			targetResult = target.NewContainerTargetResult(nil, &artifact)
			destination := jobTarget.ImageName

			logWithId.Printf("[container] 📦 Preparing upload to '%s'", destination)

			client, err := impl.getContainerClient(destination, targetOptions)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			logWithId.Printf("[container] ⬆ Uploading the image to %s", client.Target.String())

			sourcePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)

			// TODO: get the container type from the metadata of the osbuild job
			sourceRef := fmt.Sprintf("oci-archive:%s", sourcePath)

			digest, err := client.UploadImage(context.Background(), sourceRef, "")

			if err != nil {
				logWithId.Infof("[container] 🙁 Upload of '%s' failed: %v", sourceRef, err)
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}
			logWithId.Printf("[container] 🎉 Image uploaded (%s)!", digest.String())
			targetResult.Options = &target.ContainerTargetResultOptions{URL: client.Target.String(), Digest: digest.String()}

		case *target.PulpOSTreeTargetOptions:
			targetResult = target.NewPulpOSTreeTargetResult(nil, &artifact)
			archivePath := filepath.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)

			client, err := impl.getPulpClient(targetOptions)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorInvalidConfig, err.Error(), nil)
				break
			}

			url, err := client.UploadAndDistributeCommit(archivePath, targetOptions.Repository, targetOptions.BasePath)
			if err != nil {
				targetResult.TargetError = clienterrors.New(clienterrors.ErrorUploadingImage, err.Error(), nil)
				break
			}
			targetResult.Options = &target.PulpOSTreeTargetResultOptions{RepoURL: url}

		default:
			// TODO: we may not want to return completely here with multiple targets, because then no TargetErrors will be added to the JobError details
			// Nevertheless, all target errors will be still in the OSBuildJobResult.
			osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorInvalidTarget, fmt.Sprintf("invalid target type: %s", jobTarget.Name), nil)
			return nil
		}

		// this is a programming error
		if targetResult == nil {
			panic("target results object not created by the target handling code")
		}
		osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, targetResult)
	}

	targetErrors := osbuildJobResult.TargetErrors()
	if len(targetErrors) != 0 {
		osbuildJobResult.JobError = clienterrors.New(clienterrors.ErrorTargetError, "at least one target failed", targetErrors)
	} else {
		osbuildJobResult.Success = true
		osbuildJobResult.UploadStatus = "success"
	}

	return nil
}

// extractXzArchive extracts the provided XZ archive in the same directory
// and returns the path to decompressed file.
func extractXzArchive(archivePath string) (string, error) {
	workingDir, archiveFilename := path.Split(archivePath)
	decompressedFilename := strings.TrimSuffix(archiveFilename, ".xz")

	cmd := exec.Command("xz", "-d", archivePath)
	cmd.Dir = workingDir
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return path.Join(workingDir, decompressedFilename), nil
}

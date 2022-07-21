package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/upload/oci"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type S3Configuration struct {
	Creds               string
	Endpoint            string
	Region              string
	Bucket              string
	CABundle            string
	SkipSSLVerification bool
}

type OSBuildJobImpl struct {
	Store             string
	Output            string
	KojiServers       map[string]kojiServer
	GCPCreds          string
	AzureCreds        *azure.Credentials
	AWSCreds          string
	AWSBucket         string
	S3Config          S3Configuration
	ContainerAuthFile string
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
		if impl.AWSBucket != "" {
			bucket = impl.AWSBucket
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
// 1. If the provided `credentials` parameter is not `nil`, it is used to
//    authenticate with GCP.
//
// 2. If a path to GCP credentials file was provided in the worker's
//    configuration, it is used to authenticate with GCP.
//
// 3. Use Application Default Credentials from the Google library, which tries
//    to automatically find a way to authenticate using the following options:
//
//    3a. If `GOOGLE_APPLICATION_CREDENTIALS` environment variable is set, it
//        tries to load and use credentials form the file pointed to by the
//        variable.
//
//    3b. It tries to authenticate using the service account attached to the
//        resource which is running the code (e.g. Google Compute Engine VM).
func (impl *OSBuildJobImpl) getGCP(credentials []byte) (*gcp.GCP, error) {
	if credentials != nil {
		logrus.Info("[GCP] 🔑 using credentials provided with the job request")
		return gcp.New(credentials)
	} else if impl.GCPCreds != "" {
		logrus.Info("[GCP] 🔑 using credentials from the worker configuration")
		return gcp.NewFromFile(impl.GCPCreds)
	} else {
		logrus.Info("[GCP] 🔑 using Application Default Credentials via Google library")
		return gcp.New(nil)
	}
}

func validateResult(result *worker.OSBuildJobResult, jobID string) {
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
		return
	} else {
		logWithId.Infof("osbuild job succeeded")
	}
	result.Success = true
}

func uploadToS3(a *awscloud.AWS, outputDirectory, exportPath, bucket, key, filename string) (string, *clienterrors.Error) {
	imagePath := path.Join(outputDirectory, exportPath, filename)

	if key == "" {
		key = uuid.New().String()
	}
	key += "-" + filename

	_, err := a.Upload(imagePath, bucket, key)
	if err != nil {
		return "", clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())

	}

	url, err := a.S3ObjectPresignedURL(bucket, key)
	if err != nil {
		return "", clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
	}

	return url, nil
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
		Arch:         common.CurrentArch(),
	}

	hostOS, err := common.GetRedHatRelease()
	if err != nil {
		osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, err.Error())
		return nil
	}
	osbuildJobResult.HostOS = hostOS

	var outputDirectory string

	// In all cases it is necessary to report result back to osbuild-composer worker API.
	defer func() {
		validateResult(osbuildJobResult, job.Id().String())

		err := job.Update(osbuildJobResult)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}

		err = os.RemoveAll(outputDirectory)
		if err != nil {
			logWithId.Errorf("Error removing temporary output directory (%s): %v", outputDirectory, err)
		}
	}()

	outputDirectory, err = ioutil.TempDir(impl.Output, job.Id().String()+"-*")
	if err != nil {
		return fmt.Errorf("error creating temporary output directory: %v", err)
	}

	// Read the job specification
	var jobArgs worker.OSBuildJob
	err = job.Args(&jobArgs)
	if err != nil {
		return err
	}

	// In case the manifest is empty, try to get it from dynamic args
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
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args")
				return err
			}

			// skip the job if the manifest generation failed
			if manifestJR.JobError != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorManifestDependency, "Manifest dependency failed")
				return nil
			}
			jobArgs.Manifest = manifestJR.Manifest
		}

		if len(jobArgs.Manifest) == 0 {
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyManifest, "Job has no manifest")
			return nil
		}
	}

	// Explicitly check that none of the job dependencies failed.
	// This covers mainly the case when there are more than one job dependencies.
	for idx := 0; idx < job.NDynamicArgs(); idx++ {
		var jobResult worker.JobResult
		err = job.DynamicArgs(idx, &jobResult)
		if err != nil {
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args")
			return err
		}

		if jobResult.JobError != nil {
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorJobDependency, "Job dependency failed")
			return nil
		}
	}

	// copy pipeline info to the result
	osbuildJobResult.PipelineNames = jobArgs.PipelineNames

	// get exports for all job's targets
	exports := jobArgs.OsbuildExports()
	if len(exports) == 0 {
		osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, "no osbuild export specified for the job")
		return nil
	}

	var extraEnv []string
	if impl.ContainerAuthFile != "" {
		extraEnv = []string{
			fmt.Sprintf("REGISTRY_AUTH_FILE=%s", impl.ContainerAuthFile),
		}
	}

	// Run osbuild and handle two kinds of errors
	osbuildJobResult.OSBuildOutput, err = osbuild.RunOSBuild(jobArgs.Manifest, impl.Store, outputDirectory, exports, nil, extraEnv, true, os.Stderr)
	// First handle the case when "running" osbuild failed
	if err != nil {
		osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "osbuild build failed")
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
		osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "osbuild build failed")
		return nil
	}

	for _, jobTarget := range jobArgs.Targets {
		var targetResult *target.TargetResult
		switch targetOptions := jobTarget.Options.(type) {
		case *target.WorkerServerTargetOptions:
			targetResult = target.NewWorkerServerTargetResult()
			var f *os.File
			imagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)
			f, err = os.Open(imagePath)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, err.Error())
				break
			}
			defer f.Close()
			err = job.UploadArtifact(jobTarget.ImageName, f)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}

		case *target.VMWareTargetOptions:
			targetResult = target.NewVMWareTargetResult()
			credentials := vmware.Credentials{
				Username:   targetOptions.Username,
				Password:   targetOptions.Password,
				Host:       targetOptions.Host,
				Cluster:    targetOptions.Cluster,
				Datacenter: targetOptions.Datacenter,
				Datastore:  targetOptions.Datastore,
			}

			tempDirectory, err := ioutil.TempDir(impl.Output, job.Id().String()+"-vmware-*")
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			defer func() {
				err := os.RemoveAll(tempDirectory)
				if err != nil {
					logWithId.Errorf("Error removing temporary directory for vmware symlink(%s): %v", tempDirectory, err)
				}
			}()

			// create a symlink so that uploaded image has the name specified by user
			imageName := jobTarget.ImageName + ".vmdk"
			imagePath := path.Join(tempDirectory, imageName)

			exportedImagePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)
			err = os.Symlink(exportedImagePath, imagePath)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			err = vmware.UploadImage(credentials, imagePath)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}

		case *target.AWSTargetOptions:
			targetResult = target.NewAWSTargetResult(nil)
			a, err := impl.getAWS(targetOptions.Region, targetOptions.AccessKeyID, targetOptions.SecretAccessKey, targetOptions.SessionToken)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			key := targetOptions.Key
			if key == "" {
				key = uuid.New().String()
			}

			bucket := targetOptions.Bucket
			if impl.AWSBucket != "" {
				bucket = impl.AWSBucket
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
					targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorTargetError, "Failed to extract compressed image", err.Error())
					break
				}
			}

			_, err = a.Upload(imagePath, bucket, key)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}

			ami, err := a.Register(jobTarget.ImageName, bucket, key, targetOptions.ShareWithAccounts, common.CurrentArch())
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, err.Error())
				break
			}

			if ami == nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, "No ami returned")
				break
			}
			targetResult.Options = &target.AWSTargetResultOptions{
				Ami:    *ami,
				Region: targetOptions.Region,
			}

		case *target.AWSS3TargetOptions:
			targetResult = target.NewAWSS3TargetResult(nil)
			a, bucket, err := impl.getAWSForS3Target(targetOptions)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			url, targetError := uploadToS3(a, outputDirectory, jobTarget.OsbuildArtifact.ExportName, bucket, targetOptions.Key, jobTarget.OsbuildArtifact.ExportFilename)
			if targetError != nil {
				targetResult.TargetError = targetError
				break
			}
			targetResult.Options = &target.AWSS3TargetResultOptions{URL: url}

		case *target.AzureTargetOptions:
			targetResult = target.NewAzureTargetResult()
			azureStorageClient, err := azure.NewStorageClient(targetOptions.StorageAccount, targetOptions.StorageAccessKey)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
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
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}

		case *target.GCPTargetOptions:
			targetResult = target.NewGCPTargetResult(nil)
			ctx := context.Background()

			g, err := impl.getGCP(targetOptions.Credentials)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			logWithId.Infof("[GCP] 🚀 Uploading image to: %s/%s", targetOptions.Bucket, targetOptions.Object)
			_, err = g.StorageObjectUpload(ctx, path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename),
				targetOptions.Bucket, targetOptions.Object, map[string]string{gcp.MetadataKeyImageName: jobTarget.ImageName})
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}

			logWithId.Infof("[GCP] 📥 Importing image into Compute Engine as '%s'", jobTarget.ImageName)

			_, importErr := g.ComputeImageInsert(ctx, targetOptions.Bucket, targetOptions.Object, jobTarget.ImageName, []string{targetOptions.Region}, gcp.GuestOsFeaturesByDistro(targetOptions.Os))
			if importErr == nil {
				logWithId.Infof("[GCP] 🎉 Image import finished successfully")
			}

			// Cleanup storage before checking for errors
			logWithId.Infof("[GCP] 🧹 Deleting uploaded image file: %s/%s", targetOptions.Bucket, targetOptions.Object)
			if err = g.StorageObjectDelete(ctx, targetOptions.Bucket, targetOptions.Object); err != nil {
				logWithId.Errorf("[GCP] Encountered error while deleting object: %v", err)
			}

			// check error from ComputeImageInsert()
			if importErr != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, importErr.Error())
				break
			}
			logWithId.Infof("[GCP] 💿 Image URL: %s", g.ComputeImageURL(jobTarget.ImageName))

			if len(targetOptions.ShareWithAccounts) > 0 {
				logWithId.Infof("[GCP] 🔗 Sharing the image with: %+v", targetOptions.ShareWithAccounts)
				err = g.ComputeImageShare(ctx, jobTarget.ImageName, targetOptions.ShareWithAccounts)
				if err != nil {
					targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorSharingTarget, err.Error())
					break
				}
			}
			targetResult.Options = &target.GCPTargetResultOptions{
				ImageName: jobTarget.ImageName,
				ProjectID: g.GetProjectID(),
			}

		case *target.AzureImageTargetOptions:
			targetResult = target.NewAzureImageTargetResult(nil)
			ctx := context.Background()

			if impl.AzureCreds == nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorSharingTarget, "osbuild job has org.osbuild.azure.image target but this worker doesn't have azure credentials")
				break
			}

			c, err := azure.NewClient(*impl.AzureCreds, targetOptions.TenantID)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, err.Error())
				break
			}
			logWithId.Info("[Azure] 🔑 Logged in Azure")

			storageAccountTag := azure.Tag{
				Name:  "imageBuilderStorageAccount",
				Value: fmt.Sprintf("location=%s", targetOptions.Location),
			}

			storageAccount, err := c.GetResourceNameByTag(
				ctx,
				targetOptions.SubscriptionID,
				targetOptions.ResourceGroup,
				storageAccountTag,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("searching for a storage account failed: %v", err))
				break
			}

			if storageAccount == "" {
				logWithId.Info("[Azure] 📦 Creating a new storage account")
				const storageAccountPrefix = "ib"
				storageAccount = azure.RandomStorageAccountName(storageAccountPrefix)

				err := c.CreateStorageAccount(
					ctx,
					targetOptions.SubscriptionID,
					targetOptions.ResourceGroup,
					storageAccount,
					targetOptions.Location,
					storageAccountTag,
				)
				if err != nil {
					targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating a new storage account failed: %v", err))
					break
				}
			}

			logWithId.Info("[Azure] 🔑📦 Retrieving a storage account key")
			storageAccessKey, err := c.GetStorageAccountKey(
				ctx,
				targetOptions.SubscriptionID,
				targetOptions.ResourceGroup,
				storageAccount,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("retrieving the storage account key failed: %v", err))
				break
			}

			azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating the storage client failed: %v", err))
				break
			}

			storageContainer := "imagebuilder"

			logWithId.Info("[Azure] 📦 Ensuring that we have a storage container")
			err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("cannot create a storage container: %v", err))
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
					targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorTargetError, "Failed to extract compressed image", err.Error())
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
				azure.DefaultUploadThreads,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, fmt.Sprintf("uploading the image failed: %v", err))
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
				targetOptions.Location,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, fmt.Sprintf("registering the image failed: %v", err))
				break
			}
			logWithId.Info("[Azure] 🎉 Image uploaded and registered!")
			targetResult.Options = &target.AzureImageTargetResultOptions{
				ImageName: jobTarget.ImageName,
			}

		case *target.KojiTargetOptions:
			targetResult = target.NewKojiTargetResult(nil)
			kojiServerURL, err := url.Parse(targetOptions.Server)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("failed to parse Koji server URL: %v", err))
				break
			}

			kojiServer, exists := impl.KojiServers[kojiServerURL.Hostname()]
			if !exists {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("Koji server has not been configured: %s", kojiServerURL.Hostname()))
				break
			}

			kojiTransport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor)

			kojiAPI, err := koji.NewFromGSSAPI(targetOptions.Server, &kojiServer.creds, kojiTransport)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("failed to authenticate with Koji server %q: %v", kojiServerURL.Hostname(), err))
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
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorKojiBuild, fmt.Sprintf("failed to open the image for reading: %v", err))
				break
			}
			defer file.Close()

			logWithId.Info("[Koji] ⬆ Uploading the image")
			imageHash, imageSize, err := kojiAPI.Upload(file, targetOptions.UploadDirectory, jobTarget.ImageName)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}
			logWithId.Info("[Koji] 🎉 Image successfully uploaded")
			targetResult.Options = &target.KojiTargetResultOptions{
				ImageMD5:  imageHash,
				ImageSize: imageSize,
			}

		case *target.OCITargetOptions:
			targetResult = target.NewOCITargetResult(nil)
			// create an ociClient uploader with a valid storage client
			var ociClient oci.Client
			ociClient, err = oci.NewClient(&oci.ClientParams{
				User:        targetOptions.User,
				Region:      targetOptions.Region,
				Tenancy:     targetOptions.Tenancy,
				Fingerprint: targetOptions.Fingerprint,
				PrivateKey:  targetOptions.PrivateKey,
			})
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}
			logWithId.Info("[OCI] 🔑 Logged in OCI")
			logWithId.Info("[OCI] ⬆ Uploading the image")
			file, err := os.Open(path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename))
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}
			defer file.Close()
			i, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			imageID, err := ociClient.Upload(
				fmt.Sprintf("osbuild-upload-%d", i),
				targetOptions.Bucket,
				targetOptions.Namespace,
				file,
				targetOptions.Compartment,
				jobTarget.ImageName,
			)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}
			logWithId.Info("[OCI] 🎉 Image uploaded and registered!")
			targetResult.Options = &target.OCITargetResultOptions{ImageID: imageID}

		case *target.ContainerTargetOptions:
			targetResult = target.NewContainerTargetResult()
			destination := jobTarget.ImageName

			logWithId.Printf("[container] ⬆ Uploading the image to %s", destination)

			ctx := context.Background()
			client, err := container.NewClient(destination)
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				break
			}

			if targetOptions.Username != "" || targetOptions.Password != "" {
				client.SetCredentials(targetOptions.Username, targetOptions.Password)
			}
			client.SetTLSVerify(targetOptions.TlsVerify)

			sourcePath := path.Join(outputDirectory, jobTarget.OsbuildArtifact.ExportName, jobTarget.OsbuildArtifact.ExportFilename)

			// TODO: get the container type from the metadata of the osbuild job
			sourceRef := fmt.Sprintf("oci-archive:%s", sourcePath)

			digest, err := client.UploadImage(ctx, sourceRef, "")
			if err != nil {
				targetResult.TargetError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				break
			}
			logWithId.Printf("[container] 🎉 Image uploaded (%s)!", digest.String())

		default:
			// TODO: we may not want to return completely here with multiple targets, because then no TargetErrors will be added to the JobError details
			// Nevertheless, all target errors will be still in the OSBuildJobResult.
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTarget, fmt.Sprintf("invalid target type: %s", jobTarget.Name))
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
		osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorTargetError, "at least one target failed", targetErrors)
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

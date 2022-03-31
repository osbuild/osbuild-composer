package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"os"
	"path"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/upload/oci"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/common"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type OSBuildJobImpl struct {
	Store       string
	Output      string
	KojiServers map[string]koji.GSSAPICredentials
	GCPCreds    []byte
	AzureCreds  *azure.Credentials
	AWSCreds    string
	AWSBucket   string
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

func (impl *OSBuildJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id().String())
	// Initialize variable needed for reporting back to osbuild-composer.
	var osbuildJobResult *worker.OSBuildJobResult = &worker.OSBuildJobResult{
		Success: false,
		OSBuildOutput: &osbuild.Result{
			Success: false,
		},
		UploadStatus: "failure",
	}

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

	outputDirectory, err := ioutil.TempDir(impl.Output, job.Id().String()+"-*")
	if err != nil {
		return fmt.Errorf("error creating temporary output directory: %v", err)
	}

	// Read the job specification
	var args worker.OSBuildJob
	err = job.Args(&args)
	if err != nil {
		return err
	}

	// In case the manifest is empty, try to get it from dynamic args
	if len(args.Manifest) == 0 {
		if job.NDynamicArgs() > 0 {
			var manifestJR worker.ManifestJobByIDResult
			err = job.DynamicArgs(0, &manifestJR)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args")
				return err
			}

			// skip the job if the manifest generation failed
			if manifestJR.JobError != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorManifestDependency, "Manifest dependency failed")
				return nil
			}
			args.Manifest = manifestJR.Manifest
			if len(args.Manifest) == 0 {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyManifest, "Received empty manifest")
				return nil
			}
		} else {
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyManifest, "Job has no manifest")
			return nil
		}
	}
	// copy pipeline info to the result
	osbuildJobResult.PipelineNames = args.PipelineNames

	// The specification allows multiple upload targets because it is an array, but we don't support it.
	// Return an error to osbuild-composer.
	if len(args.Targets) > 1 {
		logrus.Warnf("The job specification contains more than one upload target. This is not supported any more. " +
			"This might indicate a deployment of incompatible osbuild-worker and osbuild-composer versions.")
		return nil
	}

	exports := args.Exports
	if len(exports) == 0 {
		// job did not define exports, likely coming from an older version of composer
		// fall back to default "assembler"
		exports = []string{"assembler"}
	} else if len(exports) > 1 {
		// this worker only supports returning one (1) export
		return fmt.Errorf("at most one build artifact can be exported")
	}

	// Run osbuild and handle two kinds of errors
	osbuildJobResult.OSBuildOutput, err = RunOSBuild(args.Manifest, impl.Store, outputDirectory, exports, os.Stderr)
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

	streamOptimizedPath := ""

	// NOTE: Currently OSBuild supports multiple exports, but this isn't used
	// by any of the image types and it can't be specified during the request.
	// Use the first (and presumably only) export for the imagePath.
	exportPath := exports[0]
	if osbuildJobResult.OSBuildOutput.Success && args.ImageName != "" {
		var f *os.File
		imagePath := path.Join(outputDirectory, exportPath, args.ImageName)
		if args.StreamOptimized {
			f, err = vmware.OpenAsStreamOptimizedVmdk(imagePath)
			if err != nil {
				return err
			}
			streamOptimizedPath = f.Name()
		} else {
			f, err = os.Open(imagePath)
			if err != nil {
				return err
			}
		}
		err = job.UploadArtifact(args.ImageName, f)
		if err != nil {
			return err
		}
		f.Close()
	}

	if len(args.Targets) == 0 {
		// There is no upload target, mark this job a success.
		osbuildJobResult.Success = true
		osbuildJobResult.UploadStatus = "success"
	} else if len(args.Targets) == 1 {
		switch options := args.Targets[0].Options.(type) {
		case *target.VMWareTargetOptions:
			credentials := vmware.Credentials{
				Username:   options.Username,
				Password:   options.Password,
				Host:       options.Host,
				Cluster:    options.Cluster,
				Datacenter: options.Datacenter,
				Datastore:  options.Datastore,
			}

			tempDirectory, err := ioutil.TempDir(impl.Output, job.Id().String()+"-vmware-*")
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}

			defer func() {
				err := os.RemoveAll(tempDirectory)
				if err != nil {
					logWithId.Errorf("Error removing temporary directory for vmware symlink(%s): %v", tempDirectory, err)
				}
			}()

			// create a symlink so that uploaded image has the name specified by user
			imageName := args.Targets[0].ImageName + ".vmdk"
			imagePath := path.Join(tempDirectory, imageName)
			err = os.Symlink(streamOptimizedPath, imagePath)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}

			err = vmware.UploadImage(credentials, imagePath)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AWSTargetOptions:
			a, err := impl.getAWS(options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}

			bucket := options.Bucket
			if impl.AWSBucket != "" {
				bucket = impl.AWSBucket
			}
			_, err = a.Upload(path.Join(outputDirectory, exportPath, options.Filename), bucket, key)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}

			ami, err := a.Register(args.Targets[0].ImageName, bucket, key, options.ShareWithAccounts, common.CurrentArch())
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, err.Error())
				return nil
			}

			if ami == nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, "No ami returned")
				return nil
			}

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAWSTargetResult(&target.AWSTargetResultOptions{
				Ami:    *ami,
				Region: options.Region,
			}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AWSS3TargetOptions:
			a, err := impl.getAWS(options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}
			key += "-" + options.Filename

			bucket := options.Bucket
			if impl.AWSBucket != "" {
				bucket = impl.AWSBucket
			}

			imagePath := path.Join(outputDirectory, exportPath, options.Filename)

			// *** SPECIAL VMDK HANDLING START ***
			// Upload the VMDK image as stream-optimized.
			// The VMDK conversion is applied only when the job was submitted by Weldr API,
			// therefore we need to do the conversion here explicitly if it was not done.
			if args.StreamOptimized {
				// If the streamOptimizedPath is empty, the conversion was not done
				if streamOptimizedPath == "" {
					var f *os.File
					f, err = vmware.OpenAsStreamOptimizedVmdk(imagePath)
					if err != nil {
						osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
						return nil
					}
					streamOptimizedPath = f.Name()
					f.Close()
				}
				// Replace the original file by the stream-optimized one
				err = os.Rename(streamOptimizedPath, imagePath)
				if err != nil {
					osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
					return nil
				}
			}
			// *** SPECIAL VMDK HANDLING END ***

			_, err = a.Upload(imagePath, bucket, key)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}
			url, err := a.S3ObjectPresignedURL(bucket, key)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAWSS3TargetResult(&target.AWSS3TargetResultOptions{URL: url}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AzureTargetOptions:
			azureStorageClient, err := azure.NewStorageClient(options.StorageAccount, options.StorageAccessKey)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return err
			}

			metadata := azure.BlobMetadata{
				StorageAccount: options.StorageAccount,
				ContainerName:  options.Container,
				BlobName:       args.Targets[0].ImageName,
			}

			const azureMaxUploadGoroutines = 4
			err = azureStorageClient.UploadPageBlob(
				metadata,
				path.Join(outputDirectory, exportPath, options.Filename),
				azureMaxUploadGoroutines,
			)

			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.GCPTargetOptions:
			ctx := context.Background()

			g, err := gcp.New(impl.GCPCreds)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}

			logWithId.Infof("[GCP] 🚀 Uploading image to: %s/%s", options.Bucket, options.Object)
			_, err = g.StorageObjectUpload(ctx, path.Join(outputDirectory, exportPath, options.Filename),
				options.Bucket, options.Object, map[string]string{gcp.MetadataKeyImageName: args.Targets[0].ImageName})
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, err.Error())
				return nil
			}

			logWithId.Infof("[GCP] 📥 Importing image into Compute Engine as '%s'", args.Targets[0].ImageName)
			imageBuild, importErr := g.ComputeImageImport(ctx, options.Bucket, options.Object, args.Targets[0].ImageName, options.Os, options.Region)
			if imageBuild != nil {
				logWithId.Infof("[GCP] 📜 Image import log URL: %s", imageBuild.LogUrl)
				logWithId.Infof("[GCP] 🎉 Image import finished with status: %s", imageBuild.Status)

				// Cleanup all resources potentially left after the image import job
				deleted, err := g.CloudbuildBuildCleanup(ctx, imageBuild.Id)
				for _, d := range deleted {
					logWithId.Infof("[GCP] 🧹 Deleted resource after image import job: %s", d)
				}
				if err != nil {
					logWithId.Errorf("[GCP] Encountered error during image import cleanup: %v", err)
				}
			}

			// Cleanup storage before checking for errors
			logWithId.Infof("[GCP] 🧹 Deleting uploaded image file: %s/%s", options.Bucket, options.Object)
			if err = g.StorageObjectDelete(ctx, options.Bucket, options.Object); err != nil {
				logWithId.Errorf("[GCP] Encountered error while deleting object: %v", err)
			}

			// check error from ComputeImageImport()
			if importErr != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, importErr.Error())
				return nil
			}
			logWithId.Infof("[GCP] 💿 Image URL: %s", g.ComputeImageURL(args.Targets[0].ImageName))

			if len(options.ShareWithAccounts) > 0 {
				logWithId.Infof("[GCP] 🔗 Sharing the image with: %+v", options.ShareWithAccounts)
				err = g.ComputeImageShare(ctx, args.Targets[0].ImageName, options.ShareWithAccounts)
				if err != nil {
					osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorSharingTarget, err.Error())
					return nil
				}
			}

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewGCPTargetResult(&target.GCPTargetResultOptions{
				ImageName: args.Targets[0].ImageName,
				ProjectID: g.GetProjectID(),
			}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AzureImageTargetOptions:
			ctx := context.Background()

			if impl.AzureCreds == nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorSharingTarget, "osbuild job has org.osbuild.azure.image target but this worker doesn't have azure credentials")
				return nil
			}

			c, err := azure.NewClient(*impl.AzureCreds, options.TenantID)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, err.Error())
				return nil
			}
			logWithId.Info("[Azure] 🔑 Logged in Azure")

			storageAccountTag := azure.Tag{
				Name:  "imageBuilderStorageAccount",
				Value: fmt.Sprintf("location=%s", options.Location),
			}

			storageAccount, err := c.GetResourceNameByTag(
				ctx,
				options.SubscriptionID,
				options.ResourceGroup,
				storageAccountTag,
			)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("searching for a storage account failed: %v", err))
				return nil
			}

			if storageAccount == "" {
				logWithId.Info("[Azure] 📦 Creating a new storage account")
				const storageAccountPrefix = "ib"
				storageAccount = azure.RandomStorageAccountName(storageAccountPrefix)

				err := c.CreateStorageAccount(
					ctx,
					options.SubscriptionID,
					options.ResourceGroup,
					storageAccount,
					options.Location,
					storageAccountTag,
				)
				if err != nil {
					osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating a new storage account failed: %v", err))
					return nil
				}
			}

			logWithId.Info("[Azure] 🔑📦 Retrieving a storage account key")
			storageAccessKey, err := c.GetStorageAccountKey(
				ctx,
				options.SubscriptionID,
				options.ResourceGroup,
				storageAccount,
			)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("retrieving the storage account key failed: %v", err))
				return nil
			}

			azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("creating the storage client failed: %v", err))
				return nil
			}

			storageContainer := "imagebuilder"

			logWithId.Info("[Azure] 📦 Ensuring that we have a storage container")
			err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, fmt.Sprintf("cannot create a storage container: %v", err))
				return nil
			}

			blobName := args.Targets[0].ImageName
			if !strings.HasSuffix(blobName, ".vhd") {
				blobName += ".vhd"
			}

			logWithId.Info("[Azure] ⬆ Uploading the image")
			err = azureStorageClient.UploadPageBlob(
				azure.BlobMetadata{
					StorageAccount: storageAccount,
					ContainerName:  storageContainer,
					BlobName:       blobName,
				},
				path.Join(outputDirectory, exportPath, options.Filename),
				azure.DefaultUploadThreads,
			)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, fmt.Sprintf("uploading the image failed: %v", err))
				return nil
			}

			logWithId.Info("[Azure] 📝 Registering the image")
			err = c.RegisterImage(
				ctx,
				options.SubscriptionID,
				options.ResourceGroup,
				storageAccount,
				storageContainer,
				blobName,
				args.Targets[0].ImageName,
				options.Location,
			)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, fmt.Sprintf("registering the image failed: %v", err))
				return nil
			}

			logWithId.Info("[Azure] 🎉 Image uploaded and registered!")

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAzureImageTargetResult(&target.AzureImageTargetResultOptions{
				ImageName: args.Targets[0].ImageName,
			}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.OCITargetOptions:
			// create an ociClient uploader with a valid storage client
			var ociClient oci.Client
			ociClient, err = oci.NewClient(&oci.ClientParams{
				User:        options.User,
				Region:      options.Region,
				Tenancy:     options.Tenancy,
				Fingerprint: options.Fingerprint,
				PrivateKey:  options.PrivateKey,
			})
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}
			log.Print("[OCI] 🔑 Logged in OCI")
			log.Print("[OCI] ⬆ Uploading the image")
			file, err := os.Open(path.Join(outputDirectory, exportPath, options.FileName))
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}
			defer file.Close()
			i, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			imageID, err := ociClient.Upload(
				fmt.Sprintf("osbuild-upload-%d", i),
				options.Bucket,
				options.Namespace,
				file,
				options.Compartment,
				args.Targets[0].ImageName,
			)
			if err != nil {
				osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidConfig, err.Error())
				return nil
			}
			log.Print("[OCI] 🎉 Image uploaded and registered!")

			osbuildJobResult.TargetResults = append(
				osbuildJobResult.TargetResults,
				target.NewOCITargetResult(&target.OCITargetResultOptions{ImageID: imageID}),
			)
			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		default:
			osbuildJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorInvalidTarget, fmt.Sprintf("invalid target type: %s", args.Targets[0].Name))
			return nil
		}
	}

	return nil
}

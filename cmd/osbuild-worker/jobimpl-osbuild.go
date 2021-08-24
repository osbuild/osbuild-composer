package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/common"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type OSBuildJobImpl struct {
	Store       string
	Output      string
	KojiServers map[string]koji.GSSAPICredentials
	GCPCreds    []byte
	AzureCreds  *azure.Credentials
}

func appendTargetError(res *worker.OSBuildJobResult, err error) {
	errStr := err.Error()
	log.Printf("target failed: %s", errStr)
	res.TargetErrors = append(res.TargetErrors, errStr)
}

func (impl *OSBuildJobImpl) Run(job worker.Job) error {
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
		err := job.Update(osbuildJobResult)
		if err != nil {
			log.Printf("Error reporting job result: %v", err)
		}

		err = os.RemoveAll(outputDirectory)
		if err != nil {
			log.Printf("Error removing temporary output directory (%s): %v", outputDirectory, err)
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
	// The specification allows multiple upload targets because it is an array, but we don't support it.
	// Return an error to osbuild-composer.
	if len(args.Targets) > 1 {
		log.Printf("The job specification contains more than one upload target. This is not supported any more. " +
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
		return err
	}

	log.Println("Build stages results:")

	// Include the build stages output inside the worker's logs.
	for _, stage := range osbuildJobResult.OSBuildOutput.Build.Stages {
		if stage.Success {
			log.Println(stage.Name, " success")
		} else {
			log.Printf("%s failure:\n", stage.Name)
			stageOutput := strings.Split(stage.Output, "\n")
			for _, line := range stageOutput {
				log.Printf("	%s", line)
			}
		}
	}

	log.Println("Stages results:")

	// Include the stages output inside the worker's logs.
	for _, stage := range osbuildJobResult.OSBuildOutput.Stages {
		if stage.Success {
			log.Println(stage.Name, " success")
		} else {
			log.Printf("%s failure:\n", stage.Name)
			stageOutput := strings.Split(stage.Output, "\n")
			for _, line := range stageOutput {
				log.Printf("	%s", line)
			}
		}
	}

	// Second handle the case when the build failed, but osbuild finished successfully
	if !osbuildJobResult.OSBuildOutput.Success {
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
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			defer func() {
				err := os.RemoveAll(tempDirectory)
				if err != nil {
					log.Printf("Error removing temporary directory for vmware symlink(%s): %v", tempDirectory, err)
				}
			}()

			// create a symlink so that uploaded image has the name specified by user
			imageName := args.Targets[0].ImageName + ".vmdk"
			imagePath := path.Join(tempDirectory, imageName)
			err = os.Symlink(streamOptimizedPath, imagePath)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			err = vmware.UploadImage(credentials, imagePath)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AWSTargetOptions:
			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}

			_, err = a.Upload(path.Join(outputDirectory, exportPath, options.Filename), options.Bucket, key)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			ami, err := a.Register(args.Targets[0].ImageName, options.Bucket, key, options.ShareWithAccounts, common.CurrentArch())
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			if ami == nil {
				appendTargetError(osbuildJobResult, fmt.Errorf("No ami returned"))
				return nil
			}

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAWSTargetResult(&target.AWSTargetResultOptions{
				Ami:    *ami,
				Region: options.Region,
			}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AWSS3TargetOptions:
			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey, options.SessionToken)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}
			key += "-" + options.Filename

			_, err = a.Upload(path.Join(outputDirectory, exportPath, options.Filename), options.Bucket, key)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}
			url, err := a.S3ObjectPresignedURL(options.Bucket, key)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAWSS3TargetResult(&target.AWSS3TargetResultOptions{URL: url}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.AzureTargetOptions:
			azureStorageClient, err := azure.NewStorageClient(options.StorageAccount, options.StorageAccessKey)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
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
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		case *target.GCPTargetOptions:
			ctx := context.Background()

			g, err := gcp.New(impl.GCPCreds)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			log.Printf("[GCP] 🚀 Uploading image to: %s/%s", options.Bucket, options.Object)
			_, err = g.StorageObjectUpload(ctx, path.Join(outputDirectory, exportPath, options.Filename),
				options.Bucket, options.Object, map[string]string{gcp.MetadataKeyImageName: args.Targets[0].ImageName})
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}

			log.Printf("[GCP] 📥 Importing image into Compute Engine as '%s'", args.Targets[0].ImageName)
			imageBuild, importErr := g.ComputeImageImport(ctx, options.Bucket, options.Object, args.Targets[0].ImageName, options.Os, options.Region)
			if imageBuild != nil {
				log.Printf("[GCP] 📜 Image import log URL: %s", imageBuild.LogUrl)
				log.Printf("[GCP] 🎉 Image import finished with status: %s", imageBuild.Status)

				// Cleanup all resources potentially left after the image import job
				deleted, err := g.CloudbuildBuildCleanup(ctx, imageBuild.Id)
				for _, d := range deleted {
					log.Printf("[GCP] 🧹 Deleted resource after image import job: %s", d)
				}
				if err != nil {
					log.Printf("[GCP] Encountered error during image import cleanup: %v", err)
				}
			}

			// Cleanup storage before checking for errors
			log.Printf("[GCP] 🧹 Deleting uploaded image file: %s/%s", options.Bucket, options.Object)
			if err = g.StorageObjectDelete(ctx, options.Bucket, options.Object); err != nil {
				log.Printf("[GCP] Encountered error while deleting object: %v", err)
			}

			// check error from ComputeImageImport()
			if importErr != nil {
				appendTargetError(osbuildJobResult, importErr)
				return nil
			}
			log.Printf("[GCP] 💿 Image URL: %s", g.ComputeImageURL(args.Targets[0].ImageName))

			if len(options.ShareWithAccounts) > 0 {
				log.Printf("[GCP] 🔗 Sharing the image with: %+v", options.ShareWithAccounts)
				err = g.ComputeImageShare(ctx, args.Targets[0].ImageName, options.ShareWithAccounts)
				if err != nil {
					appendTargetError(osbuildJobResult, err)
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
				appendTargetError(osbuildJobResult, fmt.Errorf("osbuild job has org.osbuild.azure.image target but this worker doesn't have azure credentials"))
				return nil
			}

			c, err := azure.NewClient(*impl.AzureCreds, options.TenantID)
			if err != nil {
				appendTargetError(osbuildJobResult, err)
				return nil
			}
			log.Print("[Azure] 🔑 Logged in Azure")

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
				appendTargetError(osbuildJobResult, fmt.Errorf("searching for a storage account failed: %v", err))
				return nil
			}

			if storageAccount == "" {
				log.Print("[Azure] 📦 Creating a new storage account")
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
					appendTargetError(osbuildJobResult, fmt.Errorf("creating a new storage account failed: %v", err))
					return nil
				}
			}

			log.Print("[Azure] 🔑📦 Retrieving a storage account key")
			storageAccessKey, err := c.GetStorageAccountKey(
				ctx,
				options.SubscriptionID,
				options.ResourceGroup,
				storageAccount,
			)
			if err != nil {
				appendTargetError(osbuildJobResult, fmt.Errorf("retrieving the storage account key failed: %v", err))
				return nil
			}

			azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
			if err != nil {
				appendTargetError(osbuildJobResult, fmt.Errorf("creating the storage client failed: %v", err))
				return nil
			}

			storageContainer := "imagebuilder"

			log.Print("[Azure] 📦 Ensuring that we have a storage container")
			err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
			if err != nil {
				appendTargetError(osbuildJobResult, fmt.Errorf("cannot create a storage container: %v", err))
				return nil
			}

			blobName := args.Targets[0].ImageName
			if !strings.HasSuffix(blobName, ".vhd") {
				blobName += ".vhd"
			}

			log.Print("[Azure] ⬆ Uploading the image")
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
				appendTargetError(osbuildJobResult, fmt.Errorf("uploading the image failed: %v", err))
				return nil
			}

			log.Print("[Azure] 📝 Registering the image")
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
				appendTargetError(osbuildJobResult, fmt.Errorf("registering the image failed: %v", err))
				return nil
			}

			log.Print("[Azure] 🎉 Image uploaded and registered!")

			osbuildJobResult.TargetResults = append(osbuildJobResult.TargetResults, target.NewAzureImageTargetResult(&target.AzureImageTargetResultOptions{
				ImageName: args.Targets[0].ImageName,
			}))

			osbuildJobResult.Success = true
			osbuildJobResult.UploadStatus = "success"
		default:
			err = fmt.Errorf("invalid target type: %s", args.Targets[0].Name)
			appendTargetError(osbuildJobResult, err)
			return nil
		}
	}

	return nil
}

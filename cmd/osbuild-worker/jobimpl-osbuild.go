package main

import (
	"context"
	"errors"
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

func packageMetadataToSignature(pkg osbuild.RPMPackageMetadata) *string {
	if pkg.SigGPG != "" {
		return &pkg.SigGPG
	} else if pkg.SigPGP != "" {
		return &pkg.SigPGP
	}
	return nil
}

func osbuildStagesToRPMs(stages []osbuild.StageResult) []koji.RPM {
	rpms := make([]koji.RPM, 0)
	for _, stage := range stages {
		switch metadata := stage.Metadata.(type) {
		case *osbuild.RPMStageMetadata:
			for _, pkg := range metadata.Packages {
				rpms = append(rpms, koji.RPM{
					Type:      "rpm",
					Name:      pkg.Name,
					Epoch:     pkg.Epoch,
					Version:   pkg.Version,
					Release:   pkg.Release,
					Arch:      pkg.Arch,
					Sigmd5:    pkg.SigMD5,
					Signature: packageMetadataToSignature(pkg),
				})
			}
		default:
			continue
		}
	}
	return rpms
}

func (impl *OSBuildJobImpl) Run(job worker.Job) error {
	// Initialize variables needed for reporting back to osbuild-composer
	var outputDirectory string
	var r []error
	var targetResults []*target.TargetResult
	var osbuildOutput *osbuild.Result = &osbuild.Result{
		Success: false,
	}

	defer func() {
		var targetErrors []string
		for _, err := range r {
			errStr := err.Error()
			fmt.Printf("target errored: %s", errStr)
			targetErrors = append(targetErrors, errStr)
		}

		var uploadstatus string = "failure"
		if len(targetErrors) == 0 {
			uploadstatus = "success"
		}

		// In all cases it is necessary to report result back to osbuild-composer worker API
		err := job.Update(&worker.OSBuildJobResult{
			Success:       osbuildOutput.Success && len(targetErrors) == 0,
			OSBuildOutput: osbuildOutput,
			TargetErrors:  targetErrors,
			TargetResults: targetResults,
			UploadStatus:  uploadstatus,
		})
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

	var args worker.OSBuildJob
	err = job.Args(&args)
	if err != nil {
		return err
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

	osbuildOutput, err = RunOSBuild(args.Manifest, impl.Store, outputDirectory, exports, os.Stderr)
	if err != nil {
		return err
	}

	streamOptimizedPath := ""

	// NOTE: Currently OSBuild supports multiple exports, but this isn't used
	// by any of the image types and it can't be specified during the request.
	// Use the first (and presumably only) export for the imagePath.
	exportPath := exports[0]
	if osbuildOutput.Success && args.ImageName != "" {
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

	for _, t := range args.Targets {
		switch options := t.Options.(type) {
		case *target.VMWareTargetOptions:
			if !osbuildOutput.Success {
				continue
			}

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
				r = append(r, err)
				continue
			}

			defer func() {
				err := os.RemoveAll(tempDirectory)
				if err != nil {
					log.Printf("Error removing temporary directory for vmware symlink(%s): %v", tempDirectory, err)
				}
			}()

			// create a symlink so that uploaded image has the name specified by user
			imageName := t.ImageName + ".vmdk"
			imagePath := path.Join(tempDirectory, imageName)
			err = os.Symlink(streamOptimizedPath, imagePath)
			if err != nil {
				r = append(r, err)
				continue
			}

			err = vmware.UploadImage(credentials, imagePath)
			if err != nil {
				r = append(r, err)
				continue
			}

		case *target.AWSTargetOptions:
			if !osbuildOutput.Success {
				continue
			}

			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey)
			if err != nil {
				r = append(r, err)
				continue
			}

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}

			_, err = a.Upload(path.Join(outputDirectory, options.Filename), options.Bucket, key)
			if err != nil {
				r = append(r, err)
				continue
			}

			ami, err := a.Register(t.ImageName, options.Bucket, key, options.ShareWithAccounts, common.CurrentArch())
			if err != nil {
				r = append(r, err)
				continue
			}

			if ami == nil {
				r = append(r, fmt.Errorf("No ami returned"))
				continue
			}

			targetResults = append(targetResults, target.NewAWSTargetResult(&target.AWSTargetResultOptions{
				Ami:    *ami,
				Region: options.Region,
			}))
		case *target.AzureTargetOptions:
			if !osbuildOutput.Success {
				continue
			}

			azureStorageClient, err := azure.NewStorageClient(options.StorageAccount, options.StorageAccessKey)
			if err != nil {
				r = append(r, err)
				continue
			}

			metadata := azure.BlobMetadata{
				StorageAccount: options.StorageAccount,
				ContainerName:  options.Container,
				BlobName:       t.ImageName,
			}

			const azureMaxUploadGoroutines = 4
			err = azureStorageClient.UploadPageBlob(
				metadata,
				path.Join(outputDirectory, options.Filename),
				azureMaxUploadGoroutines,
			)

			if err != nil {
				r = append(r, err)
				continue
			}
		case *target.GCPTargetOptions:
			if !osbuildOutput.Success {
				continue
			}

			g, err := gcp.New(impl.GCPCreds)
			if err != nil {
				r = append(r, err)
				continue
			}

			log.Printf("[GCP] 🚀 Uploading image to: %s/%s", options.Bucket, options.Object)
			_, err = g.StorageObjectUpload(path.Join(outputDirectory, options.Filename),
				options.Bucket, options.Object, map[string]string{gcp.MetadataKeyImageName: t.ImageName})
			if err != nil {
				r = append(r, err)
				continue
			}

			log.Printf("[GCP] 📥 Importing image into Compute Node as '%s'", t.ImageName)
			imageBuild, importErr := g.ComputeImageImport(options.Bucket, options.Object, t.ImageName, options.Os, options.Region)
			if imageBuild != nil {
				log.Printf("[GCP] 📜 Image import log URL: %s", imageBuild.LogUrl)
				log.Printf("[GCP] 🎉 Image import finished with status: %s", imageBuild.Status)
			}

			// Cleanup storage before checking for errors
			log.Printf("[GCP] 🧹 Deleting uploaded image file: %s/%s", options.Bucket, options.Object)
			if err = g.StorageObjectDelete(options.Bucket, options.Object); err != nil {
				log.Printf("[GCP] Encountered error while deleting object: %v", err)
			}

			deleted, errs := g.StorageImageImportCleanup(t.ImageName)
			for _, d := range deleted {
				log.Printf("[GCP] 🧹 Deleted image import job file '%s'", d)
			}
			for _, e := range errs {
				log.Printf("[GCP] Encountered error during image import cleanup: %v", e)
			}

			// check error from ComputeImageImport()
			if importErr != nil {
				r = append(r, importErr)
				continue
			}
			log.Printf("[GCP] 💿 Image URL: %s", g.ComputeImageURL(t.ImageName))

			if len(options.ShareWithAccounts) > 0 {
				log.Printf("[GCP] 🔗 Sharing the image with: %+v", options.ShareWithAccounts)
				err = g.ComputeImageShare(t.ImageName, options.ShareWithAccounts)
				if err != nil {
					r = append(r, err)
					continue
				}
			}

			targetResults = append(targetResults, target.NewGCPTargetResult(&target.GCPTargetResultOptions{
				ImageName: t.ImageName,
				ProjectID: g.GetProjectID(),
			}))

		case *target.AzureImageTargetOptions:
			ctx := context.Background()

			if impl.AzureCreds == nil {
				r = append(r, errors.New("osbuild job has org.osbuild.azure.image target but this worker doesn't have azure credentials"))
				continue
			}

			c, err := azure.NewClient(*impl.AzureCreds, options.TenantID)
			if err != nil {
				r = append(r, err)
				continue
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
				r = append(r, fmt.Errorf("searching for a storage account failed: %v", err))
				continue
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
					r = append(r, fmt.Errorf("creating a new storage account failed: %v", err))
					continue
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
				r = append(r, fmt.Errorf("retrieving the storage account key failed: %v", err))
				continue
			}

			azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
			if err != nil {
				r = append(r, fmt.Errorf("creating the storage client failed: %v", err))
				continue
			}

			storageContainer := "imagebuilder"

			log.Print("[Azure] 📦 Ensuring that we have a storage container")
			err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
			if err != nil {
				r = append(r, fmt.Errorf("cannot create a storage container: %v", err))
				continue
			}

			blobName := t.ImageName
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
				path.Join(outputDirectory, options.Filename),
				azure.DefaultUploadThreads,
			)
			if err != nil {
				r = append(r, fmt.Errorf("uploading the image failed: %v", err))
				continue
			}

			log.Print("[Azure] 📝 Registering the image")
			err = c.RegisterImage(
				ctx,
				options.SubscriptionID,
				options.ResourceGroup,
				storageAccount,
				storageContainer,
				blobName,
				t.ImageName,
				options.Location,
			)
			if err != nil {
				r = append(r, fmt.Errorf("registering the image failed: %v", err))
				continue
			}

			log.Print("[Azure] 🎉 Image uploaded and registered!")

			targetResults = append(targetResults, target.NewAzureImageTargetResult(&target.AzureImageTargetResultOptions{
				ImageName: t.ImageName,
			}))
		default:
			r = append(r, fmt.Errorf("invalid target type"))
		}
	}

	return nil
}

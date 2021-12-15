package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/sirupsen/logrus"
)

type strArrayFlag []string

func (a *strArrayFlag) String() string {
	return fmt.Sprintf("%+v", []string(*a))
}

func (a *strArrayFlag) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func main() {

	var credentialsPath string
	var bucketName string
	var objectName string
	var region string
	var osFamily string
	var imageName string
	var imageFile string
	var shareWith strArrayFlag

	var skipUpload bool
	var skipImport bool

	flag.StringVar(&credentialsPath, "cred-path", "", "Path to a file with service account credentials")
	flag.StringVar(&bucketName, "bucket", "", "Target Storage Bucket name")
	flag.StringVar(&objectName, "object", "", "Target Storage Object name")
	flag.StringVar(&region, "region", "", "Target region for the uploaded image")
	flag.StringVar(&osFamily, "os", "", "OS type used to determine which version of GCP guest tools to install")
	flag.StringVar(&imageName, "image-name", "", "Image name after import to Compute Engine")
	flag.StringVar(&imageFile, "image", "", "Image file to upload")
	flag.Var(&shareWith, "share-with", "Accounts to share the image with. Can be set multiple times. Allowed values are 'user:{emailid}' / 'serviceAccount:{emailid}' / 'group:{emailid}' / 'domain:{domain}'.")
	flag.BoolVar(&skipUpload, "skip-upload", false, "Use to skip Image Upload step")
	flag.BoolVar(&skipImport, "skip-import", false, "Use to skup Image Import step")
	flag.Parse()

	var credentials []byte
	if credentialsPath != "" {
		var err error
		credentials, err = ioutil.ReadFile(credentialsPath)
		if err != nil {
			logrus.Fatalf("[GCP] Error while reading credentials: %v", err)
		}
	}

	g, err := gcp.New(credentials)
	if err != nil {
		logrus.Fatalf("[GCP] Failed to create new GCP object: %v", err)
	}

	ctx := context.Background()

	// Upload image to the Storage
	if !skipUpload {
		logrus.Infof("[GCP] 🚀 Uploading image to: %s/%s", bucketName, objectName)
		_, err := g.StorageObjectUpload(ctx, imageFile, bucketName, objectName,
			map[string]string{gcp.MetadataKeyImageName: imageName})
		if err != nil {
			logrus.Fatalf("[GCP] Uploading image failed: %v", err)
		}
	}

	// Import Image to Compute Engine
	if !skipImport {
		logrus.Infof("[GCP] 📥 Importing image into Compute Engine as '%s'", imageName)
		imageBuild, importErr := g.ComputeImageImport(ctx, bucketName, objectName, imageName, osFamily, region)
		if imageBuild != nil {
			logrus.Infof("[GCP] 📜 Image import log URL: %s", imageBuild.LogUrl)
			logrus.Infof("[GCP] 🎉 Image import finished with status: %s", imageBuild.Status)

			// Cleanup all resources potentially left after the image import job
			deleted, err := g.CloudbuildBuildCleanup(ctx, imageBuild.Id)
			for _, d := range deleted {
				logrus.Infof("[GCP] 🧹 Deleted resource after image import job: %s", d)
			}
			if err != nil {
				logrus.Warnf("[GCP] Encountered error during image import cleanup: %v", err)
			}
		}

		// Cleanup storage before checking for errors
		logrus.Infof("[GCP] 🧹 Deleting uploaded image file: %s/%s", bucketName, objectName)
		if err = g.StorageObjectDelete(ctx, bucketName, objectName); err != nil {
			logrus.Warnf("[GCP] Encountered error while deleting object: %v", err)
		}

		// check error from ComputeImageImport()
		if importErr != nil {
			logrus.Fatalf("[GCP] Importing image failed: %v", importErr)
		}
		logrus.Infof("[GCP] 💿 Image URL: %s", g.ComputeImageURL(imageName))
	}

	// Share the imported Image with specified accounts using IAM policy
	if len(shareWith) > 0 {
		logrus.Infof("[GCP] 🔗 Sharing the image with: %+v", shareWith)
		err = g.ComputeImageShare(ctx, imageName, []string(shareWith))
		if err != nil {
			logrus.Fatalf("[GCP] Sharing image failed: %s", err)
		}
	}
}

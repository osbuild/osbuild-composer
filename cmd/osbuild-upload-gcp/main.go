package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
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
	flag.StringVar(&imageName, "image-name", "", "Image name after import to Compute Node")
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
			log.Fatalf("[GCP] Error while reading credentials: %v", err)
		}
	}

	g, err := gcp.New(credentials)
	if err != nil {
		log.Fatalf("[GCP] Failed to create new GCP object: %v", err)
	}

	// Upload image to the Storage
	if !skipUpload {
		log.Printf("[GCP] 🚀 Uploading image to: %s/%s", bucketName, objectName)
		_, err := g.StorageObjectUpload(imageFile, bucketName, objectName,
			map[string]string{gcp.MetadataKeyImageName: imageName})
		if err != nil {
			log.Fatalf("[GCP] Uploading image failed: %v", err)
		}
	}

	// Import Image to Compute Node
	if !skipImport {
		log.Printf("[GCP] 📥 Importing image into Compute Node as '%s'", imageName)
		imageBuild, importErr := g.ComputeImageImport(bucketName, objectName, imageName, osFamily, region)
		if imageBuild != nil {
			log.Printf("[GCP] 📜 Image import log URL: %s", imageBuild.LogUrl)
			log.Printf("[GCP] 🎉 Image import finished with status: %s", imageBuild.Status)
		}

		// Cleanup storage before checking for errors
		log.Printf("[GCP] 🧹 Deleting uploaded image file: %s/%s", bucketName, objectName)
		if err = g.StorageObjectDelete(bucketName, objectName); err != nil {
			log.Printf("[GCP] Encountered error while deleting object: %v", err)
		}

		deleted, errs := g.StorageImageImportCleanup(imageName)
		for _, d := range deleted {
			log.Printf("[GCP] 🧹 Deleted image import job file '%s'", d)
		}
		for _, e := range errs {
			log.Printf("[GCP] Encountered error during image import cleanup: %v", e)
		}

		// check error from ComputeImageImport()
		if importErr != nil {
			log.Fatalf("[GCP] Importing image failed: %v", err)
		}
		log.Printf("[GCP] 💿 Image URL: %s", g.ComputeImageURL(imageName))
	}

	// Share the imported Image with specified accounts using IAM policy
	if len(shareWith) > 0 {
		log.Printf("[GCP] 🔗 Sharing the image with: %+v", shareWith)
		err = g.ComputeImageShare(imageName, []string(shareWith))
		if err != nil {
			log.Fatalf("[GCP] Sharing image failed: %s", err)
		}
	}
}

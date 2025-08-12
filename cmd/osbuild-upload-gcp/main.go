package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/osbuild/images/pkg/cloud/gcp"
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
	var regions strArrayFlag
	var osFamily string
	var imageName string
	var imageFile string
	var shareWith strArrayFlag

	var skipUpload bool
	var skipImport bool

	flag.StringVar(&credentialsPath, "cred-path", "", "Path to a file with service account credentials")
	flag.StringVar(&bucketName, "bucket", "", "Target Storage Bucket name")
	flag.StringVar(&objectName, "object", "", "Target Storage Object name")
	flag.Var(&regions, "regions", "Target regions for the uploaded image")
	flag.StringVar(&osFamily, "os-family", "rhel-8", "OS family to determine Guest OS features when importing the image.")
	flag.StringVar(&imageName, "image-name", "", "Image name after import to Compute Engine")
	flag.StringVar(&imageFile, "image", "", "Image file to upload")
	flag.Var(&shareWith, "share-with", "Accounts to share the image with. Can be set multiple times. Allowed values are 'user:{emailid}' / 'serviceAccount:{emailid}' / 'group:{emailid}' / 'domain:{domain}'.")
	flag.BoolVar(&skipUpload, "skip-upload", false, "Use to skip Image Upload step")
	flag.BoolVar(&skipImport, "skip-import", false, "Use to skip Image Import step")
	flag.Parse()

	var guestOSFeatures []*computepb.GuestOsFeature

	switch osFamily {
	case "rhel-8":
		guestOSFeatures = gcp.GuestOsFeaturesRHEL8
	case "rhel-9":
		guestOSFeatures = gcp.GuestOsFeaturesRHEL9
	default:
		log.Fatalf("[GCP] Unknown OS Family %q. Use one of: 'rhel-8', 'rhel-9'.", osFamily)
	}

	var credentials []byte
	if credentialsPath != "" {
		var err error
		credentials, err = os.ReadFile(credentialsPath)
		if err != nil {
			log.Fatalf("[GCP] Error while reading credentials: %v", err)
		}
	}

	g, err := gcp.New(credentials)
	if err != nil {
		log.Fatalf("[GCP] Failed to create new GCP object: %v", err)
	}

	ctx := context.Background()

	// Upload image to the Storage
	if !skipUpload {
		log.Printf("[GCP] 🚀 Uploading image to: %s/%s", bucketName, objectName)
		_, err := g.StorageObjectUpload(ctx, imageFile, bucketName, objectName,
			map[string]string{gcp.MetadataKeyImageName: imageName})
		if err != nil {
			log.Fatalf("[GCP] Uploading image failed: %v", err)
		}
	}

	// Import Image to Compute Engine
	if !skipImport {
		log.Printf("[GCP] 📥 Importing image into Compute Engine as '%s'", imageName)
		_, importErr := g.ComputeImageInsert(ctx, bucketName, objectName, imageName, regions, guestOSFeatures)

		// Cleanup storage before checking for errors
		log.Printf("[GCP] 🧹 Deleting uploaded image file: %s/%s", bucketName, objectName)
		if err = g.StorageObjectDelete(ctx, bucketName, objectName); err != nil {
			log.Printf("[GCP] Encountered error while deleting object: %v", err)
		}

		// check error from ComputeImageImport()
		if importErr != nil {
			log.Fatalf("[GCP] Importing image failed: %v", importErr)
		}
		log.Printf("[GCP] 💿 Image URL: %s", g.ComputeImageURL(imageName))
	}

	// Share the imported Image with specified accounts using IAM policy
	if len(shareWith) > 0 {
		log.Printf("[GCP] 🔗 Sharing the image with: %+v", shareWith)
		err = g.ComputeImageShare(ctx, imageName, []string(shareWith))
		if err != nil {
			log.Fatalf("[GCP] Sharing image failed: %s", err)
		}
	}
}

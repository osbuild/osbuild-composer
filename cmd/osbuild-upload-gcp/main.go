package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/osbuild/osbuild-composer/internal/upload/gcp"
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
			fmt.Fprintf(os.Stderr, "Error while reading credentials: %s\n", err)
			return
		}
	}

	g, err := gcp.New(credentials)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	// Upload image to the Storage
	if !skipUpload {
		if err := g.Upload(imageFile, bucketName, objectName); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}

	// Import Image to Compute Node
	if !skipImport {
		err = g.Import(bucketName, objectName, imageName, osFamily, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}

	// Share the imported Image with specified accounts using IAM policy
	if len(shareWith) > 0 {
		err = g.Share(imageName, []string(shareWith))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func main() {
	var accessKeyID string
	var secretAccessKey string
	var sessionToken string
	var region string
	var endpoint string
	var caBundle string
	var skipSSLVerification bool
	var bucketName string
	var keyName string
	var filename string
	var public bool
	flag.StringVar(&accessKeyID, "access-key-id", "", "access key ID")
	flag.StringVar(&secretAccessKey, "secret-access-key", "", "secret access key")
	flag.StringVar(&sessionToken, "session-token", "", "session token")
	flag.StringVar(&region, "region", "", "target region")
	flag.StringVar(&endpoint, "endpoint", "", "target endpoint")
	flag.StringVar(&caBundle, "ca-bundle", "", "path to CA bundle for the S3 server")
	flag.BoolVar(&skipSSLVerification, "skip-ssl-verification", false, "Skip the verification of the server SSL certificate")
	flag.StringVar(&bucketName, "bucket", "", "target S3 bucket name")
	flag.StringVar(&keyName, "key", "", "target S3 key name")
	flag.StringVar(&filename, "image", "", "image file to upload")
	flag.BoolVar(&public, "public", false, "if set, the S3 object is marked as public (default: false)")
	flag.Parse()

	a, err := awscloud.NewForEndpoint(endpoint, region, accessKeyID, secretAccessKey, sessionToken, caBundle, skipSSLVerification)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	uploadOutput, err := a.Upload(filename, bucketName, keyName)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	if public {
		err := a.MarkS3ObjectAsPublic(bucketName, keyName)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	fmt.Printf("file uploaded to %s\n", uploadOutput.Location)
}

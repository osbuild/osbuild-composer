package main

import (
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func main() {
	var accessKeyID string
	var secretAccessKey string
	var sessionToken string
	var region string
	var endpoint string
	var bucketName string
	var keyName string
	var filename string
	flag.StringVar(&accessKeyID, "access-key-id", "", "access key ID")
	flag.StringVar(&secretAccessKey, "secret-access-key", "", "secret access key")
	flag.StringVar(&sessionToken, "session-token", "", "session token")
	flag.StringVar(&region, "region", "", "target region")
	flag.StringVar(&endpoint, "endpoint", "", "target endpoint")
	flag.StringVar(&bucketName, "bucket", "", "target S3 bucket name")
	flag.StringVar(&keyName, "key", "", "target S3 key name")
	flag.StringVar(&filename, "image", "", "image file to upload")
	flag.Parse()

	a, err := awscloud.NewForEndpoint(endpoint, region, accessKeyID, secretAccessKey, sessionToken)
	if err != nil {
		println(err.Error())
		return
	}

	uploadOutput, err := a.Upload(filename, bucketName, keyName)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("file uploaded to %s\n", aws.StringValue(&uploadOutput.Location))
}

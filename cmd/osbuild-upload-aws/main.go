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
	var bucketName string
	var keyName string
	var filename string
	var imageName string
	var shareWith string
	var arch string
	flag.StringVar(&accessKeyID, "access-key-id", "", "access key ID")
	flag.StringVar(&secretAccessKey, "secret-access-key", "", "secret access key")
	flag.StringVar(&sessionToken, "session-token", "", "session token")
	flag.StringVar(&region, "region", "", "target region")
	flag.StringVar(&bucketName, "bucket", "", "target S3 bucket name")
	flag.StringVar(&keyName, "key", "", "target S3 key name")
	flag.StringVar(&filename, "image", "", "image file to upload")
	flag.StringVar(&imageName, "name", "", "AMI name")
	flag.StringVar(&shareWith, "account-id", "", "account id to share image with")
	flag.StringVar(&arch, "arch", "", "arch (x86_64 or aarch64)")
	flag.Parse()

	a, err := awscloud.New(region, accessKeyID, secretAccessKey, sessionToken)
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

	var share []string
	if shareWith != "" {
		share = append(share, shareWith)
	}
	ami, err := a.Register(imageName, bucketName, keyName, share, arch, nil)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("AMI registered: %s\n", aws.StringValue(ami))
}

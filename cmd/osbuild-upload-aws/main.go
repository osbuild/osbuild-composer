package main

import (
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/osbuild/osbuild-composer/internal/awsupload"
)

func main() {
	var accessKeyID string
	var secretAccessKey string
	var region string
	var bucketName string
	var keyName string
	var filename string
	var imageName string
	flag.StringVar(&accessKeyID, "access-key-id", "", "access key ID")
	flag.StringVar(&secretAccessKey, "secret-access-key", "", "secret access key")
	flag.StringVar(&region, "region", "", "target region")
	flag.StringVar(&bucketName, "bucket", "", "target S3 bucket name")
	flag.StringVar(&keyName, "key", "", "target S3 key name")
	flag.StringVar(&filename, "image", "", "image file to upload")
	flag.StringVar(&imageName, "name", "", "AMI name")
	flag.Parse()

	a, err := awsupload.New(region, accessKeyID, secretAccessKey)
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

	ami, err := a.Register(imageName, bucketName, keyName)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("AMI registered: %s\n", aws.StringValue(ami))
}

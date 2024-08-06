package main

import (
	"flag"
	"fmt"

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
	var bootMode string
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
	flag.StringVar(&bootMode, "boot-mode", "", "boot mode (legacy-bios, uefi, uefi-preferred)")
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

	fmt.Printf("file uploaded to %s\n", uploadOutput.Location)

	var share []string
	if shareWith != "" {
		share = append(share, shareWith)
	}

	var bootModePtr *string
	if bootMode != "" {
		bootModePtr = &bootMode
	}

	ami, err := a.Register(imageName, bucketName, keyName, share, arch, bootModePtr)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("AMI registered: %s\n", *ami)
}

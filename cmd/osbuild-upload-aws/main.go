package main

import (
	"flag"
	"fmt"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/common"
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
	var archOpt string
	var bootModeOpt string
	flag.StringVar(&accessKeyID, "access-key-id", "", "access key ID")
	flag.StringVar(&secretAccessKey, "secret-access-key", "", "secret access key")
	flag.StringVar(&sessionToken, "session-token", "", "session token")
	flag.StringVar(&region, "region", "", "target region")
	flag.StringVar(&bucketName, "bucket", "", "target S3 bucket name")
	flag.StringVar(&keyName, "key", "", "target S3 key name")
	flag.StringVar(&filename, "image", "", "image file to upload")
	flag.StringVar(&imageName, "name", "", "AMI name")
	flag.StringVar(&shareWith, "account-id", "", "account id to share image with")
	flag.StringVar(&archOpt, "arch", "", "arch (x86_64 or aarch64)")
	flag.StringVar(&bootModeOpt, "boot-mode", "", "boot mode (legacy-bios, uefi, uefi-preferred)")
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

	imgArch, err := arch.FromString(archOpt)
	if err != nil {
		println(err.Error())
		return
	}

	var bootMode *platform.BootMode
	switch bootModeOpt {
	case string(ec2types.BootModeValuesLegacyBios):
		bootMode = common.ToPtr(platform.BOOT_LEGACY)
	case string(ec2types.BootModeValuesUefi):
		bootMode = common.ToPtr(platform.BOOT_UEFI)
	case string(ec2types.BootModeValuesUefiPreferred):
		bootMode = common.ToPtr(platform.BOOT_HYBRID)
	case "":
		// do nothing
	default:
		println("Unknown boot mode %q, must be one of: legacy-bios, uefi, uefi-preferred", bootModeOpt)
		return
	}

	ami, _, err := a.Register(imageName, bucketName, keyName, share, imgArch, bootMode, nil)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("AMI registered: %s\n", ami)
}

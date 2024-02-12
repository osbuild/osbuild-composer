package main

import (
	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"time"
)

func main() {
	region, err := awscloud.RegionFromInstanceMetadata()
	if err != nil {
		panic(err)
	}

	aws, err := awscloud.NewDefault(region)
	if err != nil {
		panic(err)
	}

	si, err := aws.RunSecureInstance("test-profile")
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 240)

	err = aws.TerminateSecureInstance(si)
	if err != nil {
		panic(err)
	}
}

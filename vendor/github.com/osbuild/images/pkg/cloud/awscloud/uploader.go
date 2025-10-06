package awscloud

import (
	"errors"
	"fmt"
	"io"
	"slices"

	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/platform"
)

type awsUploader struct {
	client awsClient

	region     string
	bucketName string
	imageName  string
	tags       [][2]string
	targetArch arch.Arch
	bootMode   *platform.BootMode
}

type UploaderOptions struct {
	TargetArch arch.Arch
	// BootMode to set for the AMI. If nil, no explicit boot mode will be set.
	BootMode *platform.BootMode
	Tags     [][2]string
}

// testing support
type awsClient interface {
	Regions() ([]string, error)
	Buckets() ([]string, error)
	CheckBucketPermission(string, s3types.Permission) (bool, error)
	UploadFromReader(io.Reader, string, string) (*s3manager.UploadOutput, error)
	Register(name, bucket, key string, tags [][2]string, shareWith []string, architecture arch.Arch, bootMode *platform.BootMode, importRole *string) (string, string, error)
	DeleteObject(string, string) error
}

var newAwsClient = func(region string) (awsClient, error) {
	return NewDefault(region)
}

func NewUploader(region, bucketName, imageName string, opts *UploaderOptions) (cloud.Uploader, error) {
	if opts == nil {
		opts = &UploaderOptions{}
	}
	client, err := newAwsClient(region)
	if err != nil {
		return nil, err
	}

	return &awsUploader{
		client:     client,
		region:     region,
		bucketName: bucketName,
		imageName:  imageName,
		tags:       opts.Tags,
		targetArch: opts.TargetArch,
		bootMode:   opts.BootMode,
	}, nil
}

var _ cloud.Uploader = &awsUploader{}

func (au *awsUploader) Check(status io.Writer) error {
	fmt.Fprintf(status, "Checking AWS region access...\n")
	regions, err := au.client.Regions()
	if err != nil {
		return fmt.Errorf("retrieving AWS regions for '%s' failed: %w", au.region, err)
	}

	if !slices.Contains(regions, au.region) {
		return fmt.Errorf("given AWS region '%s' not found", au.region)
	}

	fmt.Fprintf(status, "Checking AWS bucket...\n")
	buckets, err := au.client.Buckets()
	if err != nil {
		return fmt.Errorf("retrieving AWS list of buckets failed: %w", err)
	}
	if !slices.Contains(buckets, au.bucketName) {
		return fmt.Errorf("bucket '%s' not found in the given AWS account", au.bucketName)
	}

	fmt.Fprintf(status, "Checking AWS bucket permissions...\n")
	writePermission, err := au.client.CheckBucketPermission(au.bucketName, s3types.PermissionWrite)
	if err != nil {
		return err
	}
	if !writePermission {
		return fmt.Errorf("you don't have write permissions to bucket '%s' with the given AWS account", au.bucketName)
	}
	fmt.Fprintf(status, "Upload conditions met.\n")
	return nil
}

func (au *awsUploader) UploadAndRegister(r io.Reader, _ uint64, status io.Writer) (err error) {
	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), au.imageName)
	fmt.Fprintf(status, "Uploading %s to %s:%s\n", au.imageName, au.bucketName, keyName)

	res, err := au.client.UploadFromReader(r, au.bucketName, keyName)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			aErr := au.client.DeleteObject(au.bucketName, keyName)
			fmt.Fprintf(status, "Deleted S3 object %s:%s\n", au.bucketName, keyName)
			err = errors.Join(err, aErr)
		}
	}()
	fmt.Fprintf(status, "File uploaded to %s\n", res.Location)
	if au.targetArch == arch.ARCH_UNSET {
		au.targetArch = arch.Current()
	}

	fmt.Fprintf(status, "Registering AMI %s\n", au.imageName)
	ami, snapshot, err := au.client.Register(au.imageName, au.bucketName, keyName, au.tags, nil, au.targetArch, au.bootMode, nil)
	if err != nil {
		return err
	}

	fmt.Fprintf(status, "Deleted S3 object %s:%s\n", au.bucketName, keyName)
	if err := au.client.DeleteObject(au.bucketName, keyName); err != nil {
		return err
	}
	fmt.Fprintf(status, "AMI registered: %s\nSnapshot ID: %s\n", ami, snapshot)
	if err != nil {
		return err
	}

	return nil
}

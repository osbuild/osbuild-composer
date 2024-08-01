package awscloud_test

import (
	"os"
	"path/filepath"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestS3MarkObjectAsPublic(t *testing.T) {
	aws := awscloud.NewForTest(nil, nil, &s3mock{t, "bucket", "object-key"}, nil, nil)
	require.NotNil(t, aws)
	require.NoError(t, aws.MarkS3ObjectAsPublic("bucket", "object-key"))
}

func TestS3Upload(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file"), []byte("imanimage"), 0600))

	aws := awscloud.NewForTest(nil, nil, nil, &s3upldrmock{t, "imanimage", "bucket", "object-key"}, nil)
	require.NotNil(t, aws)
	_, err := aws.Upload(filepath.Join(tmpDir, "file"), "bucket", "object-key")
	require.NoError(t, err)
}

func TestS3ObjectPresignedURL(t *testing.T) {
	aws := awscloud.NewForTest(nil, nil, nil, nil, &s3signmock{t, "bucket", "object-key"})
	require.NotNil(t, aws)
	url, err := aws.S3ObjectPresignedURL("bucket", "object-key")
	require.NoError(t, err)
	require.Equal(t, "https://url.real", url)
}

func TestEC2Register(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil, &s3mock{t, "bucket", "object-key"}, nil, nil)
	require.NotNil(t, aws)

	// Image without share
	imageId, err := aws.Register("image-name", "bucket", "object-key", []string{}, "x86_64", common.ToPtr("uefi-preferred"))
	require.NoError(t, err)
	require.Equal(t, "image-id", *imageId)
	// basic image import operations
	require.Equal(t, 1, m.calledFn["ImportSnapshot"])
	require.Equal(t, 1, m.calledFn["RegisterImage"])
	// sharing operations
	require.Equal(t, 0, m.calledFn["ModifyImageAttribute"])
	require.Equal(t, 0, m.calledFn["ModifySnapshotAttribute"])

	// Image with share
	imageId, err = aws.Register("image-name", "bucket", "object-key", []string{"share-with-user"}, "x86_64", common.ToPtr("uefi-preferred"))
	require.NoError(t, err)
	require.Equal(t, "image-id", *imageId)
	// basic image import operations
	require.Equal(t, 2, m.calledFn["ImportSnapshot"])
	require.Equal(t, 2, m.calledFn["RegisterImage"])
	// sharing operations
	require.Equal(t, 1, m.calledFn["ModifyImageAttribute"])
	require.Equal(t, 1, m.calledFn["ModifySnapshotAttribute"])

	// 2 snapshots, 2 images
	require.Equal(t, 4, m.calledFn["CreateTags"])
}

func TestEC2CopyImage(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil, &s3mock{t, "bucket", "object-key"}, nil, nil)
	imageId, err := aws.CopyImage("image-name", "image-id", "region")
	require.NoError(t, err)
	require.Equal(t, "image-id", imageId)
	require.Equal(t, 1, m.calledFn["CopyImage"])
	// 1 snapshot, 1 image
	require.Equal(t, 2, m.calledFn["CreateTags"])
}

func TestEC2RemoveSnapshotAndDeregisterImage(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil, &s3mock{t, "bucket", "object-key"}, nil, nil)
	require.NotNil(t, aws)

	err := aws.RemoveSnapshotAndDeregisterImage(&ec2types.Image{
		ImageId: &m.imageId,
		State:   ec2types.ImageStateAvailable,
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{
			{
				Ebs: &ec2types.EbsBlockDevice{
					SnapshotId: &m.snapshotId,
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, m.calledFn["DeleteSnapshot"])
	require.Equal(t, 1, m.calledFn["DeregisterImage"])
}

func TestEC2Regions(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil, &s3mock{t, "bucket", "object-key"}, nil, nil)
	require.NotNil(t, aws)
	regions, err := aws.Regions()
	require.NoError(t, err)
	require.NotEmpty(t, regions)
	require.Equal(t, 1, m.calledFn["DescribeRegions"])
}

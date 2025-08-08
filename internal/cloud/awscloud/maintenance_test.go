package awscloud_test

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func TestEC2RemoveSnapshotAndDeregisterImage(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil)
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

package awscloud_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func TestEC2CopyImage(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, nil, &s3mock{t, "bucket", "object-key"}, nil)
	imageId, err := aws.CopyImage("image-name", "image-id", "region")
	require.NoError(t, err)
	require.Equal(t, "image-id", imageId)
	require.Equal(t, 1, m.calledFn["CopyImage"])
	// 1 snapshot, 1 image
	require.Equal(t, 2, m.calledFn["CreateTags"])
}

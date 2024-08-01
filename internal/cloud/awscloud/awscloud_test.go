package awscloud_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func TestS3MarkObjectAsPublic(t *testing.T) {
	aws := awscloud.NewForTest(&s3mock{t, "bucket", "object-key"}, nil, nil)
	require.NotNil(t, aws)
	require.NoError(t, aws.MarkS3ObjectAsPublic("bucket", "object-key"))
}

func TestS3Upload(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file"), []byte("imanimage"), 0600))

	aws := awscloud.NewForTest(nil, &s3upldrmock{t, "imanimage", "bucket", "object-key"}, nil)
	require.NotNil(t, aws)
	_, err := aws.Upload(filepath.Join(tmpDir, "file"), "bucket", "object-key")
	require.NoError(t, err)
}

func TestS3ObjectPresignedURL(t *testing.T) {
	aws := awscloud.NewForTest(nil, nil, &s3signmock{t, "bucket", "object-key"})
	require.NotNil(t, aws)
	url, err := aws.S3ObjectPresignedURL("bucket", "object-key")
	require.NoError(t, err)
	require.Equal(t, "https://url.real", url)
}

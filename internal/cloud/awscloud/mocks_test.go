package awscloud_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

type s3mock struct {
	t *testing.T

	bucket string
	key    string
}

func (m *s3mock) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, optfns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, nil
}

func (m *s3mock) PutObjectAcl(ctx context.Context, input *s3.PutObjectAclInput, optfns ...func(*s3.Options)) (*s3.PutObjectAclOutput, error) {
	require.Equal(m.t, m.bucket, *input.Bucket)
	require.Equal(m.t, m.key, *input.Key)
	require.Equal(m.t, s3types.ObjectCannedACL(s3types.ObjectCannedACLPublicRead), input.ACL)
	return nil, nil
}

type s3upldrmock struct {
	t *testing.T

	contents string
	bucket   string
	key      string
}

func (m *s3upldrmock) Upload(ctx context.Context, input *s3.PutObjectInput, optfns ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	body, err := io.ReadAll(input.Body)
	require.NoError(m.t, err)
	require.Equal(m.t, m.contents, string(body))
	require.Equal(m.t, m.bucket, *input.Bucket)
	require.Equal(m.t, m.key, *input.Key)
	return nil, nil
}

type s3signmock struct {
	t *testing.T

	bucket string
	key    string
}

func (m *s3signmock) PresignGetObject(ctx context.Context, input *s3.GetObjectInput, optfns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	require.Equal(m.t, m.bucket, *input.Bucket)
	require.Equal(m.t, m.key, *input.Key)

	opts := &s3.PresignOptions{}
	for _, fn := range optfns {
		fn(opts)
	}
	require.Equal(m.t, time.Duration(7*24*time.Hour), opts.Expires)

	return &v4.PresignedHTTPRequest{
		URL: "https://url.real",
	}, nil
}

package awscloud

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 interface {
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	PutObjectAcl(context.Context, *s3.PutObjectAclInput, ...func(*s3.Options)) (*s3.PutObjectAclOutput, error)
}

type S3Manager interface {
	Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type S3Presign interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

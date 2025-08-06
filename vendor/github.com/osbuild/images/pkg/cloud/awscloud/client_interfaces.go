package awscloud

import (
	"context"
	"time"

	awsSigner "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ec2Client interface {
	DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)

	// Security Groups
	AuthorizeSecurityGroupIngress(context.Context, *ec2.AuthorizeSecurityGroupIngressInput, ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	CreateSecurityGroup(context.Context, *ec2.CreateSecurityGroupInput, ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	DeleteSecurityGroup(context.Context, *ec2.DeleteSecurityGroupInput, ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)

	// Instances
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	RunInstances(context.Context, *ec2.RunInstancesInput, ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput, ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)

	// Images
	RegisterImage(context.Context, *ec2.RegisterImageInput, ...func(*ec2.Options)) (*ec2.RegisterImageOutput, error)
	DeregisterImage(context.Context, *ec2.DeregisterImageInput, ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	DescribeImages(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	ModifyImageAttribute(context.Context, *ec2.ModifyImageAttributeInput, ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error)

	// Snapshots
	DeleteSnapshot(context.Context, *ec2.DeleteSnapshotInput, ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	DescribeImportSnapshotTasks(context.Context, *ec2.DescribeImportSnapshotTasksInput, ...func(*ec2.Options)) (*ec2.DescribeImportSnapshotTasksOutput, error)
	ImportSnapshot(context.Context, *ec2.ImportSnapshotInput, ...func(*ec2.Options)) (*ec2.ImportSnapshotOutput, error)
	ModifySnapshotAttribute(context.Context, *ec2.ModifySnapshotAttributeInput, ...func(*ec2.Options)) (*ec2.ModifySnapshotAttributeOutput, error)

	// Tags
	CreateTags(context.Context, *ec2.CreateTagsInput, ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

type snapshotImportedWaiterEC2 interface {
	WaitForOutput(ctx context.Context, params *ec2.DescribeImportSnapshotTasksInput, maxWaitDur time.Duration, optFns ...func(*ec2.SnapshotImportedWaiterOptions)) (*ec2.DescribeImportSnapshotTasksOutput, error)
}

type instanceRunningWaiterEC2 interface {
	Wait(ctx context.Context, params *ec2.DescribeInstancesInput, maxWaitDur time.Duration, optFns ...func(*ec2.InstanceRunningWaiterOptions)) error
}

type instanceTerminatedWaiterEC2 interface {
	Wait(ctx context.Context, params *ec2.DescribeInstancesInput, maxWaitDur time.Duration, optFns ...func(*ec2.InstanceTerminatedWaiterOptions)) error
}

type s3Client interface {
	GetBucketAcl(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	PutObjectAcl(context.Context, *s3.PutObjectAclInput, ...func(*s3.Options)) (*s3.PutObjectAclOutput, error)
}

type s3Uploader interface {
	Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type s3Presign interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*awsSigner.PresignedHTTPRequest, error)
}

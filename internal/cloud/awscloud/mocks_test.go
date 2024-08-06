package awscloud_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	require.Equal(m.t, m.bucket, *input.Bucket)
	require.Equal(m.t, m.key, *input.Key)
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

type ec2imdsmock struct {
	t *testing.T

	instanceID string
	region     string
}

func (m *ec2imdsmock) GetInstanceIdentityDocument(ctx context.Context, input *imds.GetInstanceIdentityDocumentInput, optfns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	return &imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			InstanceID: m.instanceID,
			Region:     m.region,
		},
	}, nil
}

type ec2mock struct {
	t *testing.T

	// Image function variables
	imageId    string
	imageName  string
	snapshotId string

	calledFn map[string]int
}

func newEc2Mock(t *testing.T) *ec2mock {
	return &ec2mock{
		t:          t,
		imageId:    "image-id",
		imageName:  "image-name",
		snapshotId: "snapshot-id",
		calledFn:   make(map[string]int),
	}
}

func (m *ec2mock) DescribeRegions(ctx context.Context, input *ec2.DescribeRegionsInput, optfns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	m.calledFn["DescribeRegions"] += 1
	return &ec2.DescribeRegionsOutput{
		Regions: []ec2types.Region{
			{
				RegionName: aws.String("region1"),
			},
			{
				RegionName: aws.String("region2"),
			},
		},
	}, nil
}

func (m *ec2mock) AuthorizeSecurityGroupIngress(ctx context.Context, input *ec2.AuthorizeSecurityGroupIngressInput, optfns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	m.calledFn["AuthorizeSecurityGroupIngress"] += 1
	return &ec2.AuthorizeSecurityGroupIngressOutput{
		Return: aws.Bool(true),
	}, nil
}

func (m *ec2mock) CreateSecurityGroup(ctx context.Context, input *ec2.CreateSecurityGroupInput, optfns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	m.calledFn["CreateSecurityGroup"] += 1
	return &ec2.CreateSecurityGroupOutput{
		GroupId: aws.String("sg-id"),
	}, nil
}

func (m *ec2mock) DeleteSecurityGroup(ctx context.Context, input *ec2.DeleteSecurityGroupInput, optfns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	m.calledFn["DeleteSecurityGroup"] += 1
	return nil, nil
}

func (m *ec2mock) DescribeSecurityGroups(ctx context.Context, input *ec2.DescribeSecurityGroupsInput, optfns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	m.calledFn["DescribeSecurityGroup"] += 1
	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []ec2types.SecurityGroup{
			{
				GroupId: aws.String("sg-id"),
				IpPermissions: []ec2types.IpPermission{
					{},
				},
				IpPermissionsEgress: []ec2types.IpPermission{
					{},
				},
			},
		},
	}, nil
}

func (m *ec2mock) CreateSubnet(ctx context.Context, input *ec2.CreateSubnetInput, optfns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	m.calledFn["CreateSubnet"] += 1
	return nil, nil
}

func (m *ec2mock) DeleteSubnet(ctx context.Context, input *ec2.DeleteSubnetInput, optfns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	m.calledFn["DeleteSubnet"] += 1
	return nil, nil
}

func (m *ec2mock) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput, optfns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	m.calledFn["DescribeSubnets"] += 1
	return &ec2.DescribeSubnetsOutput{
		Subnets: []ec2types.Subnet{
			{
				SubnetId: aws.String("subnet-id"),
			},
		},
	}, nil
}

func (m *ec2mock) CreateLaunchTemplate(ctx context.Context, input *ec2.CreateLaunchTemplateInput, optfns ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error) {
	m.calledFn["CreateLaunchTemplate"] += 1
	return &ec2.CreateLaunchTemplateOutput{
		LaunchTemplate: &ec2types.LaunchTemplate{
			LaunchTemplateId: aws.String("lt-id"),
		},
	}, nil
}

func (m *ec2mock) DeleteLaunchTemplate(ctx context.Context, input *ec2.DeleteLaunchTemplateInput, optfns ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error) {
	m.calledFn["DeleteLaunchTemplate"] += 1
	return nil, nil
}

func (m *ec2mock) DescribeLaunchTemplates(ctx context.Context, input *ec2.DescribeLaunchTemplatesInput, optfns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	m.calledFn["DescribeLaunchTemplates"] += 1
	return &ec2.DescribeLaunchTemplatesOutput{
		LaunchTemplates: []ec2types.LaunchTemplate{
			{
				LaunchTemplateId: aws.String("lt-id"),
			},
		},
	}, nil
}

func (m *ec2mock) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, optfns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	m.calledFn["DescribeInstances"] += 1

	// For the waiters sometimes a running instance is required, sometimes a terminated one
	state := ec2types.InstanceStateNameRunning
	if m.calledFn["DescribeInstances"]%2 == 0 {
		state = ec2types.InstanceStateNameTerminated
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: aws.String("instance-id"),
						VpcId:      aws.String("vpc-id"),
						ImageId:    aws.String("image-id"),
						SubnetId:   aws.String("subnet-id"),
						State: &ec2types.InstanceState{
							Name: state,
						},
					},
				},
			},
		},
	}, nil
}

func (m *ec2mock) DescribeInstanceStatus(ctx context.Context, input *ec2.DescribeInstanceStatusInput, optfns ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	m.calledFn["DescribeInstanceStatus"] += 1

	// For the waiters sometimes a running instance is required, sometimes a terminated one
	state := ec2types.InstanceStateNameRunning
	if m.calledFn["DescribeInstanceStatus"]%2 == 0 {
		state = ec2types.InstanceStateNameTerminated
	}

	return &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("instance-id"),
				InstanceState: &ec2types.InstanceState{
					Name: state,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}, nil
}

func (m *ec2mock) RunInstances(ctx context.Context, input *ec2.RunInstancesInput, optfns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	m.calledFn["RunInstances"] += 1
	return nil, nil
}

func (m *ec2mock) TerminateInstances(ctx context.Context, input *ec2.TerminateInstancesInput, optfns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	m.calledFn["TerminateInstances"] += 1
	return nil, nil
}

func (m *ec2mock) CreateFleet(ctx context.Context, input *ec2.CreateFleetInput, optfns ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error) {
	m.calledFn["CreateFleet"] += 1
	return &ec2.CreateFleetOutput{
		FleetId: aws.String("fleet-id"),
		Instances: []ec2types.CreateFleetInstance{
			{
				InstanceIds: []string{
					"instance-id",
				},
			},
		},
	}, nil
}

func (m *ec2mock) DeleteFleets(ctx context.Context, input *ec2.DeleteFleetsInput, optfns ...func(*ec2.Options)) (*ec2.DeleteFleetsOutput, error) {
	m.calledFn["DeleteFleets"] += 1
	return &ec2.DeleteFleetsOutput{
		UnsuccessfulFleetDeletions: nil,
		SuccessfulFleetDeletions: []ec2types.DeleteFleetSuccessItem{
			{
				FleetId: aws.String("fleet-id"),
			},
		},
	}, nil
}

func (m *ec2mock) CopyImage(ctx context.Context, input *ec2.CopyImageInput, optfns ...func(*ec2.Options)) (*ec2.CopyImageOutput, error) {
	m.calledFn["CopyImage"] += 1
	return &ec2.CopyImageOutput{
		ImageId: &m.imageId,
	}, nil
}

func (m *ec2mock) RegisterImage(ctx context.Context, input *ec2.RegisterImageInput, optfns ...func(*ec2.Options)) (*ec2.RegisterImageOutput, error) {
	m.calledFn["RegisterImage"] += 1
	return &ec2.RegisterImageOutput{
		ImageId: &m.imageId,
	}, nil
}

func (m *ec2mock) DeregisterImage(ctx context.Context, input *ec2.DeregisterImageInput, optfns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
	m.calledFn["DeregisterImage"] += 1
	return nil, nil
}

func (m *ec2mock) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput, optfns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	m.calledFn["DescribeImages"] += 1
	return &ec2.DescribeImagesOutput{
		Images: []ec2types.Image{
			{
				ImageId: &m.imageId,
				State:   ec2types.ImageStateAvailable,
				BlockDeviceMappings: []ec2types.BlockDeviceMapping{
					{
						Ebs: &ec2types.EbsBlockDevice{
							SnapshotId: &m.snapshotId,
						},
					},
				},
			},
		},
	}, nil
}

func (m *ec2mock) ModifyImageAttribute(ctx context.Context, input *ec2.ModifyImageAttributeInput, optfns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
	m.calledFn["ModifyImageAttribute"] += 1
	return nil, nil
}

func (m *ec2mock) DeleteSnapshot(ctx context.Context, input *ec2.DeleteSnapshotInput, optfns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	m.calledFn["DeleteSnapshot"] += 1
	return nil, nil
}

func (m *ec2mock) DescribeImportSnapshotTasks(ctx context.Context, input *ec2.DescribeImportSnapshotTasksInput, optfns ...func(*ec2.Options)) (*ec2.DescribeImportSnapshotTasksOutput, error) {
	m.calledFn["DescribeImportSnapshotTasks"] += 1
	return &ec2.DescribeImportSnapshotTasksOutput{
		ImportSnapshotTasks: []ec2types.ImportSnapshotTask{
			{
				ImportTaskId: aws.String("import-task-id"),
				SnapshotTaskDetail: &ec2types.SnapshotTaskDetail{
					SnapshotId: &m.snapshotId,
					Status:     aws.String("completed"),
				},
			},
		},
	}, nil
}

func (m *ec2mock) ImportSnapshot(ctx context.Context, input *ec2.ImportSnapshotInput, optfns ...func(*ec2.Options)) (*ec2.ImportSnapshotOutput, error) {
	m.calledFn["ImportSnapshot"] += 1
	return &ec2.ImportSnapshotOutput{
		ImportTaskId: aws.String("import-task-id"),
	}, nil
}

func (m *ec2mock) ModifySnapshotAttribute(ctx context.Context, input *ec2.ModifySnapshotAttributeInput, optfns ...func(*ec2.Options)) (*ec2.ModifySnapshotAttributeOutput, error) {
	m.calledFn["ModifySnapshotAttribute"] += 1
	return nil, nil
}

func (m *ec2mock) CreateTags(ctx context.Context, input *ec2.CreateTagsInput, optfns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	m.calledFn["CreateTags"] += 1
	return nil, nil
}

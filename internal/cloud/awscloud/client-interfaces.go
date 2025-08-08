package awscloud

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type EC2 interface {
	// Security Groups
	AuthorizeSecurityGroupIngress(context.Context, *ec2.AuthorizeSecurityGroupIngressInput, ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	CreateSecurityGroup(context.Context, *ec2.CreateSecurityGroupInput, ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	DeleteSecurityGroup(context.Context, *ec2.DeleteSecurityGroupInput, ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)

	// Subnets
	CreateSubnet(context.Context, *ec2.CreateSubnetInput, ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	DeleteSubnet(context.Context, *ec2.DeleteSubnetInput, ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	// LaunchTemplates
	CreateLaunchTemplate(context.Context, *ec2.CreateLaunchTemplateInput, ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error)
	DeleteLaunchTemplate(context.Context, *ec2.DeleteLaunchTemplateInput, ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error)
	DescribeLaunchTemplates(context.Context, *ec2.DescribeLaunchTemplatesInput, ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error)

	// Instances
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceStatus(context.Context, *ec2.DescribeInstanceStatusInput, ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error)
	RunInstances(context.Context, *ec2.RunInstancesInput, ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput, ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)

	// Fleets
	CreateFleet(context.Context, *ec2.CreateFleetInput, ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error)
	DeleteFleets(context.Context, *ec2.DeleteFleetsInput, ...func(*ec2.Options)) (*ec2.DeleteFleetsOutput, error)

	// Images
	CopyImage(context.Context, *ec2.CopyImageInput, ...func(*ec2.Options)) (*ec2.CopyImageOutput, error)
	DeregisterImage(context.Context, *ec2.DeregisterImageInput, ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	DescribeImages(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	ModifyImageAttribute(context.Context, *ec2.ModifyImageAttributeInput, ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error)

	// Snapshots
	DeleteSnapshot(context.Context, *ec2.DeleteSnapshotInput, ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	DescribeImportSnapshotTasks(context.Context, *ec2.DescribeImportSnapshotTasksInput, ...func(*ec2.Options)) (*ec2.DescribeImportSnapshotTasksOutput, error)
	ModifySnapshotAttribute(context.Context, *ec2.ModifySnapshotAttributeInput, ...func(*ec2.Options)) (*ec2.ModifySnapshotAttributeOutput, error)

	// Tags
	CreateTags(context.Context, *ec2.CreateTagsInput, ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

type EC2Imds interface {
	GetInstanceIdentityDocument(context.Context, *imds.GetInstanceIdentityDocumentInput, ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

type ASG interface {
	DescribeAutoScalingInstances(context.Context, *autoscaling.DescribeAutoScalingInstancesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error)
	SetInstanceProtection(context.Context, *autoscaling.SetInstanceProtectionInput, ...func(*autoscaling.Options)) (*autoscaling.SetInstanceProtectionOutput, error)
}

type S3 interface {
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	PutObjectAcl(context.Context, *s3.PutObjectAclInput, ...func(*s3.Options)) (*s3.PutObjectAclOutput, error)
}

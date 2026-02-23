package awscloud

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

func (a *AWS) ASGSetProtectHost(protect bool) error {
	identity, err := a.ec2imds.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return err
	}

	descrASG, err := a.asg.DescribeAutoScalingInstances(
		context.Background(),
		&autoscaling.DescribeAutoScalingInstancesInput{
			InstanceIds: []string{
				identity.InstanceID,
			},
		},
	)
	if err != nil {
		return err
	}

	if len(descrASG.AutoScalingInstances) == 0 {
		return nil
	}

	_, err = a.asg.SetInstanceProtection(
		context.Background(),
		&autoscaling.SetInstanceProtectionInput{
			AutoScalingGroupName: descrASG.AutoScalingInstances[0].AutoScalingGroupName,
			InstanceIds: []string{
				identity.InstanceID,
			},
			ProtectedFromScaleIn: aws.Bool(protect),
		},
	)

	return err
}

func (a *AWS) SetInstanceToUnhealthy() error {
	identity, err := a.ec2imds.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return err
	}

	_, err = a.asg.SetInstanceHealth(
		context.Background(),
		&autoscaling.SetInstanceHealthInput{
			InstanceId: &identity.InstanceID,
		},
	)
	return err
}

// Helper function to shut down instance inside of an ASG
func ShutdownSelf() error {
	region, err := RegionFromInstanceMetadata()
	if err != nil {
		return fmt.Errorf("Unable to get region from instance metadata: %w", err)
	}

	aws, err := NewDefault(region)
	if err != nil {
		return fmt.Errorf("Unable to get default aws client: %w", err)
	}

	identity, err := aws.ec2imds.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return fmt.Errorf("Unable to get identity document of instance: %w", err)
	}

	err = aws.SetInstanceToUnhealthy()
	if err != nil {
		return fmt.Errorf("Unable to set instance to unhealthy: %w", err)
	}

	_, err = aws.asg.TerminateInstanceInAutoScalingGroup(
		context.Background(),
		&autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId: &identity.InstanceID,
		},
	)
	return fmt.Errorf("Unable to terminate instance in ASG: %w", err)
}

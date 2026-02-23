package awscloud

import (
	"context"

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

func (a *AWS) ShutdownSelf() error {
	identity, err := a.ec2imds.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return err
	}

	_, err = a.asg.TerminateInstanceInAutoScalingGroup(
		context.Background(),
		&autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId: &identity.InstanceID,
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

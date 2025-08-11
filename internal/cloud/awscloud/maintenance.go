package awscloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/sirupsen/logrus"
)

// For service maintenance images are discovered by the "Name:composer-api-*" tag filter. Currently
// all image names in the service are generated, so they're guaranteed to be unique as well. If
// users are ever allowed to name their images, an extra tag should be added.
func (a *AWS) DescribeImagesByTag(tagKey, tagValue string) ([]ec2types.Image, error) {
	imgs, err := a.ec2.DescribeImages(
		context.Background(),
		&ec2.DescribeImagesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
					Values: []string{tagValue},
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return imgs.Images, nil
}

func (a *AWS) RemoveSnapshotAndDeregisterImage(image *ec2types.Image) error {
	if image == nil {
		return fmt.Errorf("image is nil")
	}

	var snapshots []*string
	for _, bdm := range image.BlockDeviceMappings {
		snapshots = append(snapshots, bdm.Ebs.SnapshotId)
	}

	_, err := a.ec2.DeregisterImage(
		context.Background(),
		&ec2.DeregisterImageInput{
			ImageId: image.ImageId,
		},
	)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		_, err = a.ec2.DeleteSnapshot(
			context.Background(),
			&ec2.DeleteSnapshotInput{
				SnapshotId: s,
			},
		)
		if err != nil {
			// TODO return err?
			logrus.Warn("Unable to remove snapshot", s)
		}
	}
	return err
}

func (a *AWS) describeInstancesByKeyValue(key, value string) ([]ec2types.Reservation, error) {
	res, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String(key),
					Values: []string{value},
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return res.Reservations, nil
}

func (a *AWS) DescribeInstancesByTag(tagKey, tagValue string) ([]ec2types.Reservation, error) {
	return a.describeInstancesByKeyValue(fmt.Sprintf("tag:%s", tagKey), tagValue)
}

func (a *AWS) DescribeInstancesBySecurityGroupID(securityGroupID string) ([]ec2types.Reservation, error) {
	return a.describeInstancesByKeyValue("instance.group-id", securityGroupID)
}

func (a *AWS) DescribeInstancesByLaunchTemplateID(launchTemplateID string) ([]ec2types.Reservation, error) {
	return a.describeInstancesByKeyValue("tag:aws:ec2launchtemplate:id", launchTemplateID)
}

func (a *AWS) DescribeInstancesByInstanceID(instanceID string) ([]ec2types.Reservation, error) {
	res, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		},
	)
	if err != nil {
		return nil, err
	}
	return res.Reservations, nil
}

func (a *AWS) DescribeSecurityGroupsByPrefix(ctx context.Context, prefix string) ([]ec2types.SecurityGroup, error) {
	var securityGroups []ec2types.SecurityGroup

	sgOutput, err := a.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return securityGroups, fmt.Errorf("failed to describe security groups: %w", err)
	}

	for _, sg := range sgOutput.SecurityGroups {
		if sg.GroupName != nil && strings.HasPrefix(*sg.GroupName, prefix) {
			securityGroups = append(securityGroups, sg)
		}
	}
	return securityGroups, nil
}

func (a *AWS) DescribeLaunchTemplatesByPrefix(ctx context.Context, prefix string) ([]ec2types.LaunchTemplate, error) {
	var launchTemplates []ec2types.LaunchTemplate

	ltOutput, err := a.ec2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
	if err != nil {
		return launchTemplates, fmt.Errorf("failed to describe security groups: %w", err)
	}

	for _, lt := range ltOutput.LaunchTemplates {
		if lt.LaunchTemplateName != nil && strings.HasPrefix(*lt.LaunchTemplateName, prefix) {
			launchTemplates = append(launchTemplates, lt)
		}
	}
	return launchTemplates, nil
}

func (a *AWS) DeleteSecurityGroupById(ctx context.Context, sgID *string) error {
	_, err := a.ec2.DeleteSecurityGroup(
		ctx,
		&ec2.DeleteSecurityGroupInput{
			GroupId: sgID,
		},
	)
	return err
}

func (a *AWS) DeleteLaunchTemplateById(ctx context.Context, ltID *string) error {
	_, err := a.ec2.DeleteLaunchTemplate(
		ctx,
		&ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: ltID,
		},
	)
	return err
}

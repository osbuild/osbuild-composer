package awscloud

import (
	"context"
	"fmt"

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

func (a *AWS) DescribeInstancesByTag(tagKey, tagValue string) ([]ec2types.Reservation, error) {
	res, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
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
	return res.Reservations, nil
}

func (a *AWS) TerminateInstances(instanceIDs []string) error {
	_, err := a.ec2.TerminateInstances(
		context.Background(),
		&ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs,
		},
	)
	return err
}

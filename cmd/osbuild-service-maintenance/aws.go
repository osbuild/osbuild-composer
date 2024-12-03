package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func AWSCleanup(maxConcurrentRequests int, dryRun bool, accessKeyID, accessKey string, cutoff time.Time) error {
	const region = "us-east-1"
	var a *awscloud.AWS
	var err error

	if accessKeyID != "" && accessKey != "" {
		a, err = awscloud.New(region, accessKeyID, accessKey, "")
		if err != nil {
			return err
		}
	} else {
		logrus.Infof("One of AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY is missing, trying default credentials…")
		a, err = awscloud.NewDefault(region)
		if err != nil {
			return err
		}
	}

	regions, err := a.Regions()
	if err != nil {
		return err
	}

	for _, region := range regions {
		a, err := awscloud.New(region, accessKeyID, accessKey, "")
		if err != nil {
			logrus.Errorf("Unable to create new aws session for region %s: %v", region, err)
			continue
		}

		var wg sync.WaitGroup
		sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
		images, err := a.DescribeImagesByTag("Name", "composer-api-*")
		if err != nil {
			logrus.Errorf("Unable to describe images for region %s: %v", region, err)
			continue
		}

		for index, image := range images {
			// TODO are these actual concerns?
			if image.ImageId == nil {
				logrus.Infof("ImageId is nil %v", image)
				continue
			}
			if image.CreationDate == nil {
				logrus.Infof("Image %v has nil creationdate", *image.ImageId)
				continue
			}

			created, err := time.Parse(time.RFC3339, *image.CreationDate)
			if err != nil {
				logrus.Infof("Unable to parse date %s for image %s", *image.CreationDate, *image.ImageId)
				continue
			}

			if !created.Before(cutoff) {
				continue
			}

			if dryRun {
				logrus.Infof("Dry run, aws image %s in region %s, with creation date %s would be removed", *image.ImageId, region, *image.CreationDate)
				continue
			}

			if err = sem.Acquire(context.Background(), 1); err != nil {
				logrus.Errorf("Error acquiring semaphore: %v", err)
				continue
			}
			wg.Add(1)

			go func(i int) {
				defer sem.Release(1)
				defer wg.Done()

				err := a.RemoveSnapshotAndDeregisterImage(&images[i])
				if err != nil {
					logrus.Errorf("Cleanup for image %s in region %s failed: %v", *images[i].ImageId, region, err)
				}
			}(index)
		}
		wg.Wait()
	}

	// Terminate leftover secure instances
	reservations, err := a.DescribeInstancesByTag("parent", "i-*")
	if err != nil {
		return fmt.Errorf("Unable to describe instances by tag %w", err)
	}

	instanceIDs := filterReservations(reservations)
	logrus.Infof("Cleaning up executor instances: %v", instanceIDs)
	if !dryRun {
		err = a.TerminateInstances(instanceIDs)
		if err != nil {
			return fmt.Errorf("Unable to terminate secure instances: %w", err)
		}
	} else {
		logrus.Info("Dry run, didn't actually terminate any instances")
	}
	return nil
}

func filterReservations(reservations []ec2types.Reservation) []string {
	var instanceIDs []string
	for _, res := range reservations {
		for _, i := range res.Instances {
			if i.LaunchTime.Before(time.Now().Add(-time.Hour * 2)) {
				instanceIDs = append(instanceIDs, *i.InstanceId)
			}
		}
	}
	return instanceIDs
}

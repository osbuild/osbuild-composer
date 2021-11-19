package main

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func AWSCleanup(maxConcurrentRequests int, accessKeyID, accessKey, region string, cutoff time.Time) error {
	a, err := awscloud.New(region, accessKeyID, accessKey, "")
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
	images, err := a.DescribeImagesByTag("Name", "composer-api-*")
	if err != nil {
		return err
	}

	for index, image := range images {
		if image.CreationDate == nil {
			logrus.Infof("Image %v has empty creationdate", image.ImageId)
			continue
		}

		created, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			logrus.Infof("Unable to parse date %v for image %v", image.CreationDate, image.ImageId)
			continue
		}

		if !created.Before(cutoff) {
			continue
		}

		wg.Add(1)
		if err = sem.Acquire(context.Background(), 1); err != nil {
			logrus.Errorf("Error acquiring semaphore: %v", err)
			continue
		}

		go func(i int) {
			defer sem.Release(1)
			defer wg.Done()

			err := a.RemoveSnapshotAndDeregisterImage(images[i])
			if err != nil {
				logrus.Errorf("Cleanup for image %s in region %s failed", images[i].ImageId, region)
			}
		}(index)
	}

	wg.Wait()
	return nil
}

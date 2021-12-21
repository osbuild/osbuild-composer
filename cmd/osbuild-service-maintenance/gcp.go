package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/compute/v1"

	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
)

func GCPCleanup(creds []byte, maxConcurrentRequests int, dryRun bool, cutoff time.Time) error {
	g, err := gcp.New(creds)
	if err != nil {
		return err
	}

	sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
	var wg sync.WaitGroup
	removeImageOlderThan := func(images *compute.ImageList) error {
		for _, image := range images.Items {
			created, err := time.Parse(time.RFC3339, image.CreationTimestamp)
			if err != nil {
				logrus.Errorf("Unable to parse image %s(%d)'s creation timestamp: %v", image.Name, image.Id, err)
				continue
			}

			if !created.Before(cutoff) {
				continue
			}

			if dryRun {
				logrus.Infof("Dry run, gcp image %s(%d), with creation date %v would be removed", image.Name, image.Id, created)
				continue
			}

			if err = sem.Acquire(context.Background(), 1); err != nil {
				logrus.Errorf("Error acquiring semaphore: %v", err)
				continue
			}
			wg.Add(1)

			go func(id string) {
				defer sem.Release(1)
				defer wg.Done()

				err = g.ComputeImageDelete(context.Background(), id)
				if err != nil {
					logrus.Errorf("Error deleting image %s created at %v", id, created)
				}
			}(fmt.Sprintf("%d", image.Id))
		}
		return nil
	}

	err = g.ComputeExecuteFunctionForImages(context.Background(), removeImageOlderThan)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}

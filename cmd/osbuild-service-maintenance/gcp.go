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

func GCPCleanup(maxConcurrentRequests int, credentials []byte, cutoff time.Time) error {
	g, err := gcp.New(credentials)
	if err != nil {
		return err
	}

	sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
	var wg sync.WaitGroup
	removeImageOlderThan := func(images *compute.ImageList) error {
		for _, image := range images.Items {
			created, err := time.Parse(time.RFC3339, image.CreationTimestamp)
			if err != nil {
				logrus.Errorf("Unable to parse creation timestamp: %v", err)
				return err // TODO continue?
			}

			if !created.Before(cutoff) {
				continue
			}

			wg.Add(1)
			if err = sem.Acquire(context.Background(), 1); err != nil {
				logrus.Errorf("Error acquiring semaphore: %v", err)
				continue
			}

			go func(id string) {
				defer sem.Release(1)
				defer wg.Done()

				err = g.ComputeImageDelete(context.Background(), id)
				if err != nil {
					logrus.Errorf("Error deleting image %s created at %v", id, created)
				}
			}(fmt.Sprintf("%s", image.Id))
		}
		return nil
	}

	err = g.ComputeExecuteForImage(context.Background(), removeImageOlderThan)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/iterator"

	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
)

func GCPCleanup(creds []byte, maxConcurrentRequests int, dryRun bool, cutoff time.Time) error {
	g, err := gcp.New(creds)
	if err != nil {
		return err
	}

	sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
	var wg sync.WaitGroup
	removeImageOlderThan := func(images *compute.ImageIterator) error {
		for {
			image, err := images.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Fatalf("Error iterating over list of images: %v", err)
			}

			created, err := time.Parse(time.RFC3339, image.GetCreationTimestamp())
			if err != nil {
				log.Printf("Unable to parse image %s(%d)'s creation timestamp: %v", image.GetName(), image.Id, err)
				continue
			}

			if !created.Before(cutoff) {
				continue
			}

			if dryRun {
				log.Printf("Dry run, gcp image %s(%d), with creation date %v would be removed", image.GetName(), image.Id, created)
				continue
			}

			if err = sem.Acquire(context.Background(), 1); err != nil {
				log.Printf("Error acquiring semaphore: %v", err)
				continue
			}
			wg.Add(1)

			go func(id string) {
				defer sem.Release(1)
				defer wg.Done()

				err = g.ComputeImageDelete(context.Background(), image.GetName())
				if err != nil {
					log.Printf("Error deleting image %s created at %v: %v", image.GetName(), created, err)
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

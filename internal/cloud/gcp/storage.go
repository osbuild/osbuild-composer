package gcp

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	// MetadataKeyImageName contains a key name used to store metadata on
	// a Storage object with the intended name of the image.
	// The metadata can be then used to associate the object with actual
	// image build using the image name.
	MetadataKeyImageName string = "osbuild-composer-image-name"
)

// StorageObjectUpload uploads an OS image to specified Cloud Storage bucket and object.
// The bucket must exist. MD5 sum of the image file and uploaded object is
// compared after the upload to verify the integrity of the uploaded image.
//
// The ObjectAttrs is returned if the object has been created.
//
// Uses:
//	- Storage API
func (g *GCP) StorageObjectUpload(filename, bucket, object string, metadata map[string]string) (*storage.ObjectAttrs, error) {
	ctx := context.Background()

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Storage client: %v", err)
	}
	defer storageClient.Close()

	// Open the image file
	imageFile, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open the image: %v", err)
	}
	defer imageFile.Close()

	// Compute MD5 checksum of the image file for later verification
	imageFileHash := md5.New()
	if _, err := io.Copy(imageFileHash, imageFile); err != nil {
		return nil, fmt.Errorf("cannot create md5 of the image: %v", err)
	}
	// Move the cursor of opened file back to the start
	if _, err := imageFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("cannot seek the image: %v", err)
	}

	// Upload the image
	// The Bucket MUST exist and be of a STANDARD storage class
	obj := storageClient.Bucket(bucket).Object(object)
	wc := obj.NewWriter(ctx)

	// Uploaded data is rejected if its MD5 hash does not match the set value.
	wc.MD5 = imageFileHash.Sum(nil)

	if metadata != nil {
		wc.ObjectAttrs.Metadata = metadata
	}

	if _, err = io.Copy(wc, imageFile); err != nil {
		return nil, fmt.Errorf("uploading the image failed: %v", err)
	}

	// The object will not be available until Close has been called.
	if err := wc.Close(); err != nil {
		return nil, fmt.Errorf("Writer.Close: %v", err)
	}

	return wc.Attrs(), nil
}

// StorageObjectDelete deletes the given object from a bucket.
//
// Uses:
//	- Storage API
func (g *GCP) StorageObjectDelete(bucket, object string) error {
	ctx := context.Background()

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Storage client: %v", err)
	}
	defer storageClient.Close()

	objectHandle := storageClient.Bucket(bucket).Object(object)
	if err = objectHandle.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete image file object: %v", err)
	}

	return nil
}

// StorageImageImportCleanup deletes all objects created as part of an Image
// import into Compute Engine and the related Build Job. The method returns a list
// of deleted Storage objects, as well as list of errors which occurred during
// the cleanup. The method tries to clean up as much as possible, therefore
// it does not return on non-fatal errors.
//
// The Build job stores a copy of the to-be-imported image in a region specific
// bucket, along with the Build job logs and some cache files.
//
// Uses:
//	- Compute Engine API
//	- Storage API
func (g *GCP) StorageImageImportCleanup(imageName string) ([]string, []error) {
	var deletedObjects []string
	var errors []error

	ctx := context.Background()

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to get Storage client: %v", err))
		return deletedObjects, errors
	}
	defer storageClient.Close()

	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to get Compute Engine client: %v", err))
		return deletedObjects, errors
	}

	// Clean up the cache bucket
	image, err := computeService.Images.Get(g.creds.ProjectID, imageName).Context(ctx).Do()
	if err != nil {
		// Without the image, we can not determine which objects to delete, just return
		errors = append(errors, fmt.Errorf("failed to get image: %v", err))
		return deletedObjects, errors
	}

	// Determine the regular expression to match files related to the specific Image Import
	// e.g. "https://www.googleapis.com/compute/v1/projects/ascendant-braid-303513/zones/europe-west1-b/disks/disk-d7tr4"
	// e.g. "https://www.googleapis.com/compute/v1/projects/ascendant-braid-303513/zones/europe-west1-b/disks/disk-l7s2w-1"
	// Needed is only the part between "disk-" and possible "-<num>"/"EOF"
	ss := strings.Split(image.SourceDisk, "/")
	srcDiskName := ss[len(ss)-1]
	ss = strings.Split(srcDiskName, "-")
	if len(ss) < 2 {
		errors = append(errors, fmt.Errorf("unexpected source disk name '%s', can not clean up storage", srcDiskName))
		return deletedObjects, errors
	}
	scrDiskSuffix := ss[1]
	// e.g. "gce-image-import-2021-02-05T17:27:40Z-2xhp5/daisy-import-image-20210205-17:27:43-s6l0l/logs/daisy.log"
	reStr := fmt.Sprintf("gce-image-import-.+-%s", scrDiskSuffix)
	cacheFilesRe := regexp.MustCompile(reStr)

	buckets := storageClient.Buckets(ctx, g.creds.ProjectID)
	for {
		bkt, err := buckets.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Errorf("failure while iterating over storage buckets: %v", err))
			return deletedObjects, errors
		}

		// Check all buckets created by the Image Import Build jobs
		// These are named e.g. "<project_id>-daisy-bkt-eu" - "ascendant-braid-303513-daisy-bkt-eu"
		if strings.HasPrefix(bkt.Name, fmt.Sprintf("%s-daisy-bkt", g.creds.ProjectID)) {
			objects := storageClient.Bucket(bkt.Name).Objects(ctx, nil)
			for {
				obj, err := objects.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					// Do not return, just log, to clean up as much as possible!
					errors = append(errors, fmt.Errorf("failure while iterating over bucket objects: %v", err))
					break
				}
				if cacheFilesRe.FindString(obj.Name) != "" {
					o := storageClient.Bucket(bkt.Name).Object(obj.Name)
					if err = o.Delete(ctx); err != nil {
						// Do not return, just log, to clean up as much as possible!
						errors = append(errors, fmt.Errorf("failed to delete storage object: %v", err))
					}
					deletedObjects = append(deletedObjects, fmt.Sprintf("%s/%s", bkt.Name, obj.Name))
				}
			}
		}
	}

	return deletedObjects, errors
}

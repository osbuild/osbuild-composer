package gcp

import (
	"context"
	// gcp uses MD5 hashes
	/* #nosec G501 */
	"crypto/md5"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
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
//   - Storage API
func (g *GCP) StorageObjectUpload(ctx context.Context, filename, bucket, object string, metadata map[string]string) (*storage.ObjectAttrs, error) {
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
	// gcp uses MD5 hashes
	/* #nosec G401 */
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
//   - Storage API
func (g *GCP) StorageObjectDelete(ctx context.Context, bucket, object string) error {
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

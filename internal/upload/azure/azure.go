package azure

import (
	"bufio"
	"bytes"
	"context"
	// Azure uses MD5 hashes
	// #gosec G501
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Credentials contains credentials to connect to your account
// It uses so called "Client credentials", see the official documentation for more information:
// https://docs.microsoft.com/en-us/azure/go/azure-sdk-go-authorization#available-authentication-types-and-methods
type Credentials struct {
	StorageAccount   string
	StorageAccessKey string
}

// ImageMetadata contains information needed to store the image in a proper place.
// In case of Azure cloud storage this includes container name and blob name.
type ImageMetadata struct {
	ContainerName string
	ImageName     string
}

// UploadImage takes the metadata and credentials required to upload the image specified by `fileName`
// It can speed up the upload by using goroutines. The number of parallel goroutines is bounded by
// the `threads` argument.
func UploadImage(credentials Credentials, metadata ImageMetadata, fileName string, threads int) error {
	// Azure cannot create an image from a storage blob without .vhd extension
	if !strings.HasSuffix(metadata.ImageName, ".vhd") {
		metadata.ImageName = metadata.ImageName + ".vhd"
	}

	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(credentials.StorageAccount, credentials.StorageAccessKey)
	if err != nil {
		return fmt.Errorf("cannot create azure credentials: %v", err)
	}

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// get storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", credentials.StorageAccount, metadata.ContainerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, p)

	// Create the container, use a never-expiring context
	ctx := context.Background()

	// Open the image file for reading
	imageFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("cannot open the image: %v", err)
	}
	defer imageFile.Close()

	// Stat image to get the file size
	stat, err := imageFile.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat the image: %v", err)
	}

	// Hash the imageFile
	// Azure uses MD5 hashes
	// #gosec G501
	imageFileHash := md5.New()
	if _, err := io.Copy(imageFileHash, imageFile); err != nil {
		return fmt.Errorf("cannot create md5 of the image: %v", err)
	}
	// Move the cursor back to the start of the imageFile
	if _, err := imageFile.Seek(0, 0); err != nil {
		return fmt.Errorf("cannot seek the image: %v", err)
	}

	// Create page blob URL. Page blob is required for VM images
	blobURL := containerURL.NewPageBlobURL(metadata.ImageName)
	_, err = blobURL.Create(ctx, stat.Size(), 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("cannot create the blob URL: %v", err)
	}
	// Wrong MD5 does not seem to have any impact on the upload
	_, err = blobURL.SetHTTPHeaders(ctx, azblob.BlobHTTPHeaders{ContentMD5: imageFileHash.Sum(nil)}, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("cannot set the HTTP headers on the blob URL: %v", err)
	}

	// Create control variables
	// This channel simulates behavior of a semaphore and bounds the number of parallel threads
	var semaphore = make(chan int, threads)
	// Forward error from goroutine to the caller
	var errorInGoroutine = make(chan error, 1)
	var counter int64 = 0

	// Create buffered reader to speed up the upload
	reader := bufio.NewReader(imageFile)
	// Run the upload
	run := true
	var wg sync.WaitGroup
	for run {
		buffer := make([]byte, azblob.PageBlobMaxUploadPagesBytes)
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				run = false
			} else {
				return fmt.Errorf("reading the image failed: %v", err)
			}
		}
		if n == 0 {
			break
		}
		wg.Add(1)
		semaphore <- 1
		go func(counter int64, buffer []byte, n int) {
			defer wg.Done()
			_, err = blobURL.UploadPages(ctx, counter*azblob.PageBlobMaxUploadPagesBytes, bytes.NewReader(buffer[:n]), azblob.PageBlobAccessConditions{}, nil)
			if err != nil {
				err = fmt.Errorf("uploading a page failed: %v", err)
				// Send the error to the error channel in a non-blocking way. If there is already an error, just discard this one
				select {
				case errorInGoroutine <- err:
				default:
				}
			}
			<-semaphore
		}(counter, buffer, n)
		counter++
	}
	// Wait for all goroutines to finish
	wg.Wait()
	// Check any errors during the transmission using a nonblocking read from the channel
	select {
	case err := <-errorInGoroutine:
		return err
	default:
	}
	// Check properties, specifically MD5 sum of the blob
	props, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("getting the properties of the new blob failed: %v", err)
	}
	var blobChecksum []byte = props.ContentMD5()
	var fileChecksum []byte = imageFileHash.Sum(nil)

	if !bytes.Equal(blobChecksum, fileChecksum) {
		return errors.New("error during image upload. the image seems to be corrupted")
	}

	return nil
}

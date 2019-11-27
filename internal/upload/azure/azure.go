package azure

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
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
// It can speed up the upload by using gorutines. The number of parallel gorutines is bounded by
// the `threads` argument.
func UploadImage(credentials Credentials, metadata ImageMetadata, fileName string, threads int) error {
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(credentials.StorageAccount, credentials.StorageAccessKey)
	if err != nil {
		return err
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
		return err
	}

	// Stat image to get the file size
	stat, err := imageFile.Stat()
	if err != nil {
		return err
	}

	// Create page blob URL. Page blob is required for VM images
	blobURL := containerURL.NewPageBlobURL(metadata.ImageName)
	_, err = blobURL.Create(ctx, stat.Size(), 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		return err
	}

	// Create control variables
	// This channel simulates behavior of a semaphore and bounds the number of parallel threads
	var semaphore = make(chan int, threads)
	// Forward error from gorutine to the caller
	var errorInGorutine = make(chan error, 1)
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
				return err
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
				// Send the error to the error channel in a non-blocking way. If there is already an error, just discart this one
				select {
				case errorInGorutine <- err:
				default:
				}
			}
			<-semaphore
		}(counter, buffer, n)
		counter++
	}
	wg.Wait()
	select {
	case err := <-errorInGorutine:
		return err
	default:
		return nil
	}
}

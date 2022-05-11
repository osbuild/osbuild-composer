package azure

import (
	"bufio"
	"bytes"
	"context"
	// azure uses MD5 hashes
	/* #nosec G501 */
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/google/uuid"
)

// StorageClient is a client for the Azure Storage API,
// see the docs: https://docs.microsoft.com/en-us/rest/api/storageservices/
type StorageClient struct {
	pipeline pipeline.Pipeline
}

// NewStorageClient creates a new client for Azure Storage API.
// See the following keys how to retrieve the storageAccessKey using the
// Azure's API:
// https://docs.microsoft.com/en-us/rest/api/storagerp/storageaccounts/listkeys
func NewStorageClient(storageAccount, storageAccessKey string) (*StorageClient, error) {
	credential, err := azblob.NewSharedKeyCredential(storageAccount, storageAccessKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create shared key credential: %v", err)
	}

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	return &StorageClient{
		pipeline: p,
	}, nil
}

// BlobMetadata contains information needed to store the image in a proper place.
// In case of Azure cloud storage this includes container name and blob name.
type BlobMetadata struct {
	StorageAccount string
	ContainerName  string
	BlobName       string
}

// DefaultUploadThreads defines a tested default value for the UploadPageBlob method's threads parameter.
const DefaultUploadThreads = 16

// UploadPageBlob takes the metadata and credentials required to upload the image specified by `fileName`
// It can speed up the upload by using goroutines. The number of parallel goroutines is bounded by
// the `threads` argument.
//
// Note that if you want to create an image out of the page blob, make sure that metadata.BlobName
// has a .vhd extension, see EnsureVHDExtension.
func (c StorageClient) UploadPageBlob(metadata BlobMetadata, fileName string, threads int) error {
	// get storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", metadata.StorageAccount, metadata.ContainerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, c.pipeline)

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

	if stat.Size()%512 != 0 {
		return errors.New("size for azure image must be aligned to 512 bytes")
	}

	// Hash the imageFile
	// azure uses MD5 hashes
	/* #nosec G401 */
	imageFileHash := md5.New()
	if _, err := io.Copy(imageFileHash, imageFile); err != nil {
		return fmt.Errorf("cannot create md5 of the image: %v", err)
	}
	// Move the cursor back to the start of the imageFile
	if _, err := imageFile.Seek(0, 0); err != nil {
		return fmt.Errorf("cannot seek the image: %v", err)
	}

	// Create page blob URL. Page blob is required for VM images
	blobURL := containerURL.NewPageBlobURL(metadata.BlobName)
	_, err = blobURL.Create(ctx, stat.Size(), 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.PremiumPageBlobAccessTierNone, azblob.BlobTagsMap{}, azblob.ClientProvidedKeyOptions{})
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
			_, err = blobURL.UploadPages(ctx, counter*azblob.PageBlobMaxUploadPagesBytes, bytes.NewReader(buffer[:n]), azblob.PageBlobAccessConditions{}, nil, azblob.ClientProvidedKeyOptions{})
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
	props, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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

// CreateStorageContainerIfNotExist creates an empty storage container inside
// a storage account. If a container with the same name already exists,
// this method is no-op.
func (c StorageClient) CreateStorageContainerIfNotExist(ctx context.Context, storageAccount, name string) error {
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", storageAccount, name))
	containerURL := azblob.NewContainerURL(*URL, c.pipeline)

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		if storageErr, ok := err.(azblob.StorageError); ok && storageErr.(azblob.StorageError).ServiceCode() == azblob.ServiceCodeContainerAlreadyExists {
			return nil
		}
		return fmt.Errorf("cannot create a storage container: %v", err)
	}

	return nil
}

// RandomStorageAccountName returns a randomly generated name that can be used
// for a storage account. This means that it must use only alphanumeric
// characters and its length must be 24 or lower.
func RandomStorageAccountName(prefix string) string {
	id := uuid.New().String()
	id = strings.ReplaceAll(id, "-", "")

	return (prefix + id)[:24]
}

// EnsureVHDExtension returns the given string with .vhd suffix if it already
// doesn't have one.
func EnsureVHDExtension(s string) string {
	if strings.HasSuffix(s, ".vhd") {
		return s
	}

	return s + ".vhd"
}

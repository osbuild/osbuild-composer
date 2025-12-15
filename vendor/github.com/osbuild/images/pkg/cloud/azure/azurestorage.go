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
	"regexp"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/pageblob"
	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
)

// StorageClient is a client for the Azure Storage API,
// see the docs: https://docs.microsoft.com/en-us/rest/api/storageservices/
type StorageClient struct {
	credential *azblob.SharedKeyCredential
}

// NewStorageClient creates a new client for Azure Storage API.
// See the following keys how to retrieve the storageAccessKey using the
// Azure's API:
// https://docs.microsoft.com/en-us/rest/api/storagerp/storageaccounts/listkeys
func NewStorageClient(storageAccount, storageAccessKey string) (*StorageClient, error) {
	credential, err := azblob.NewSharedKeyCredential(storageAccount, storageAccessKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create shared key credential: %w", err)
	}

	return &StorageClient{
		credential: credential,
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

// PageBlobMaxUploadPagesBytes defines how much bytes can we upload in a single UploadPages call.
// See https://learn.microsoft.com/en-us/rest/api/storageservices/put-page
const PageBlobMaxUploadPagesBytes = 4 * datasizes.MiB

// UploadPageBlob takes the metadata and credentials required to upload the image specified by `fileName`
// It can speed up the upload by using goroutines. The number of parallel goroutines is bounded by
// the `threads` argument.
//
// Note that if you want to create an image out of the page blob, make sure that metadata.BlobName
// has a .vhd extension, see EnsureVHDExtension.
func (c StorageClient) UploadPageBlob(metadata BlobMetadata, fileName string, threads int) error {
	// Create a page blob client.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", metadata.StorageAccount, metadata.ContainerName, metadata.BlobName))
	client, err := pageblob.NewClientWithSharedKeyCredential(URL.String(), c.credential, nil)
	if err != nil {
		return fmt.Errorf("cannot create a pageblob client: %w", err)
	}

	// Create the container, use a never-expiring context
	ctx := context.Background()

	// Open the image file for reading
	imageFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("cannot open the image: %w", err)
	}
	defer imageFile.Close()

	// Stat image to get the file size
	stat, err := imageFile.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat the image: %w", err)
	}

	if stat.Size()%512 != 0 {
		return errors.New("size for azure image must be aligned to 512 bytes")
	}

	// Hash the imageFile
	// azure uses MD5 hashes
	/* #nosec G401 */
	imageFileHash := md5.New()
	if _, err := io.Copy(imageFileHash, imageFile); err != nil {
		return fmt.Errorf("cannot create md5 of the image: %w", err)
	}
	// Move the cursor back to the start of the imageFile
	if _, err := imageFile.Seek(0, 0); err != nil {
		return fmt.Errorf("cannot seek the image: %w", err)
	}

	// Create page blob. Page blob is required for VM images
	_, err = client.Create(ctx, stat.Size(), &pageblob.CreateOptions{
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentMD5: imageFileHash.Sum(nil),
		},
	})
	if err != nil {
		return fmt.Errorf("cannot create a new page blob: %w", err)
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
	zeros := make([]byte, PageBlobMaxUploadPagesBytes)
	for run {
		buffer := make([]byte, PageBlobMaxUploadPagesBytes)
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				run = false
			} else {
				return fmt.Errorf("reading the image failed: %w", err)
			}
		}
		if n == 0 {
			break
		}

		// Skip the uploading part if there are only zeros in the buffer.
		// We already defined the size of the blob in the initial call and the blob is zero-initialized,
		// so this pushing zeros would actually be a no-op.
		// Using bytes.Equal with a preallocated buffer can be significantly faster on
		// certain hardware than just iterating over the entire slice.
		if bytes.Equal(zeros[:n], buffer[:n]) {
			counter++
			continue
		}

		wg.Add(1)
		semaphore <- 1
		go func(counter int64, buffer []byte, n int) {
			defer wg.Done()
			uploadRange := blob.HTTPRange{
				Offset: counter * PageBlobMaxUploadPagesBytes,
				Count:  int64(n),
			}
			_, err := client.UploadPages(ctx, common.NopSeekCloser(bytes.NewReader(buffer[:n])), uploadRange, nil)
			if err != nil {
				err = fmt.Errorf("uploading a page failed: %w", err)
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

	return nil
}

// CreateStorageContainerIfNotExist creates an empty storage container inside
// a storage account. If a container with the same name already exists,
// this method is no-op.
func (c StorageClient) CreateStorageContainerIfNotExist(ctx context.Context, storageAccount, name string) error {
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", storageAccount, name))

	cl, err := container.NewClientWithSharedKeyCredential(URL.String(), c.credential, nil)
	if err != nil {
		return fmt.Errorf("cannot create a storage container client: %w", err)
	}

	_, err = cl.Create(ctx, nil)
	if err != nil {
		if bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			return nil
		}

		return fmt.Errorf("cannot create a storage container: %w", err)
	}

	return nil
}

// Taken from https://docs.microsoft.com/en-us/rest/api/storageservices/set-blob-tags#request-body
var tagKeyRegexp = regexp.MustCompile(`^[a-zA-Z0-9 +-./:=_]{1,256}$`)
var tagValueRegexp = regexp.MustCompile(`^[a-zA-Z0-9 +-./:=_]{0,256}$`)

func (c StorageClient) TagBlob(ctx context.Context, metadata BlobMetadata, tags map[string]string) error {
	for key, value := range tags {
		if !tagKeyRegexp.MatchString(key) {
			return fmt.Errorf("tag key `%s` doesn't match the format accepted by Azure", key)
		}
		if !tagValueRegexp.MatchString(key) {
			return fmt.Errorf("tag value `%s` of key `%s` doesn't match the format accepted by Azure", value, key)
		}
	}

	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", metadata.StorageAccount, metadata.ContainerName, metadata.BlobName))

	client, err := blob.NewClientWithSharedKeyCredential(URL.String(), c.credential, nil)
	if err != nil {
		return fmt.Errorf("cannot create a blob client: %w", err)
	}

	_, err = client.SetTags(ctx, tags, nil)
	if err != nil {
		return fmt.Errorf("cannot tag the blob: %w", err)
	}

	return nil
}

func (c StorageClient) DeleteBlob(ctx context.Context, metadata BlobMetadata) error {
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", metadata.StorageAccount, metadata.ContainerName, metadata.BlobName))
	client, err := blob.NewClientWithSharedKeyCredential(URL.String(), c.credential, nil)
	if err != nil {
		return fmt.Errorf("cannot create a blob client: %w", err)
	}
	_, err = client.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot delete the blob: %w", err)
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

package azure

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

func handleErrors(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func loadEnvVar(t *testing.T, envVarName string) (string, bool) {
	variable, exists := os.LookupEnv(envVarName)
	if !exists {
		t.Logf("Environment variable does not exist: %s", envVarName)
		return "", false
	}
	return variable, true
}

func TestAzure_FileUpload(t *testing.T) {
	// Load configuration
	storageAccount, saExists := loadEnvVar(t, "AZURE_STORAGE_ACCOUNT")
	storageAccessKey, sakExists := loadEnvVar(t, "AZURE_STORAGE_ACCESS_KEY")
	containerName, cnExists := loadEnvVar(t, "AZURE_STORAGE_CONTAINER")
	fileName := "/tmp/testing-image.vhd"
	var threads int = 4

	// If non of the variables is set, just ignore the test
	if saExists == false && sakExists == false && cnExists == false {
		t.Log("No AZURE configuration provided, assuming that this is running in CI. Skipping the test.")
		return
	}
	// If only one/two of them are not set, then fail
	if saExists == false || sakExists == false || cnExists == false {
		t.Fatal("You need to define all variables for AZURE connection.")
	}

	// Prepare the file
	cmd := exec.Command("dd", "bs=512", "count=512", "if=/dev/urandom", fmt.Sprintf("of=%s", fileName))
	err := cmd.Run()
	handleErrors(t, err)
	t.Log("Image to upload is:", fileName)

	// Calculate MD5 sum of the file
	f, err := os.Open(fileName)
	handleErrors(t, err)

	h := md5.New()
	_, err = io.Copy(h, f)
	handleErrors(t, err)
	imageChecksum := h.Sum(nil)
	err = f.Close()
	handleErrors(t, err)

	credentials := Credentials{
		StorageAccount:   storageAccount,
		StorageAccessKey: storageAccessKey,
	}
	metadata := ImageMetadata{
		ImageName:     path.Base(fileName),
		ContainerName: containerName,
	}
	// Upload the image
	err = UploadImage(credentials, metadata, fileName, threads)
	handleErrors(t, err)

	// Download the image
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(credentials.StorageAccount, credentials.StorageAccessKey)
	handleErrors(t, err)

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// get storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", credentials.StorageAccount, metadata.ContainerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, p)

	// Create the container, use a never-expiring context
	ctx := context.Background()

	blobURL := containerURL.NewPageBlobURL(metadata.ImageName)

	get, err := blobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	handleErrors(t, err)
	blobData := &bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	_, err = blobData.ReadFrom(reader)
	handleErrors(t, err)
	reader.Close() // The client must close the response body when finished with it
	blobBytes := blobData.Bytes()
	blobChecksum := md5.Sum(blobBytes)
	t.Logf("Local image checksum:      %x\n", imageChecksum)
	t.Logf("Downloaded blob checksum : %x\n", blobChecksum)

	// Delete the blob and throw away any errors
	_, _ = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	_ = os.Remove(fileName)

	if !bytes.Equal(imageChecksum, blobChecksum[:]) {
		t.Fatalf("Checksums do not match! Local file: %x, cloud blob: %x", imageChecksum, blobChecksum[:])
	}

}

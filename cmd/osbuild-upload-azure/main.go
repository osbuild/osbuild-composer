package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

func handleErrors(err error) {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok { // This error is a Service-specific
			switch serr.ServiceCode() { // Compare serviceCode to ServiceCodeXxx constants
			case azblob.ServiceCodeContainerAlreadyExists:
				// This error is not fatal
				fmt.Println("Received 409. Container already exists")
				return
			}
		}
		// All other error causes the program to exit
		fmt.Println(err)
		os.Exit(1)
	}
}

type azureCredentials struct {
	storageAccount   string
	storageAccessKey string
}

type azureImageMetadata struct {
	containerName string
	imageName     string
}

func azureUploadImage(credentials azureCredentials, metadata azureImageMetadata, fileName string, threads int) {
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(credentials.storageAccount, credentials.storageAccessKey)
	handleErrors(err)
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// get storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", credentials.storageAccount, metadata.containerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, p)

	// Create the container, use a never-expiring context
	ctx := context.Background()

	// Open the image file for reading
	imageFile, err := os.Open(fileName)
	handleErrors(err)

	// Stat image to get the file size
	stat, err := imageFile.Stat()
	handleErrors(err)

	// Create page blob URL. Page blob is required for VM images
	blobURL := containerURL.NewPageBlobURL(metadata.imageName)
	_, err = blobURL.Create(ctx, stat.Size(), 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	handleErrors(err)

	// Create control variables
	// This channel simulates behavior of a semaphore and bounds the number of parallel threads
	var semaphore = make(chan int, threads)
	var counter int64 = 0

	// Create buffered reader to speed up the upload
	reader := bufio.NewReader(imageFile)
	imageSize := stat.Size()
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
				panic(err)
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
				log.Fatal(err)
			}
			<-semaphore
		}(counter, buffer, n)
		fmt.Printf("\rProgress: uploading bytest %d-%d from %d bytes", counter*azblob.PageBlobMaxUploadPagesBytes, counter*azblob.PageBlobMaxUploadPagesBytes+int64(n), imageSize)
		counter++
	}
	wg.Wait()
}

func checkStringNotEmpty(variable string, errorMessage string) {
	if variable == "" {
		log.Fatal(errorMessage)
	}
}

func main() {
	var storageAccount string
	var storageAccessKey string
	var fileName string
	var containerName string
	var threads int
	flag.StringVar(&storageAccount, "storage-account", "", "Azure storage account (mandatory)")
	flag.StringVar(&storageAccessKey, "storage-access-key", "", "Azure storage access key (mandatory)")
	flag.StringVar(&fileName, "image", "", "image to upload (mandatory)")
	flag.StringVar(&containerName, "container", "", "name of storage container (see Azure docs for explanation, mandatory)")
	flag.IntVar(&threads, "threads", 16, "number of threads for parallel upload")
	flag.Parse()

	checkStringNotEmpty(storageAccount, "You need to specify storage account")
	checkStringNotEmpty(storageAccessKey, "You need to specify storage access key")
	checkStringNotEmpty(fileName, "You need to specify image file")
	checkStringNotEmpty(containerName, "You need to specify container name")

	fmt.Println("Image to upload is:", fileName)

	azureUploadImage(azureCredentials{
		storageAccount:   storageAccount,
		storageAccessKey: storageAccessKey,
	}, azureImageMetadata{
		imageName:     path.Base(fileName),
		containerName: containerName,
	}, fileName, threads)

}

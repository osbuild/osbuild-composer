package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/upload/azure"
)

func checkStringNotEmpty(variable string, errorMessage string) {
	if variable == "" {
		fmt.Fprintln(os.Stderr, errorMessage)
		flag.Usage()
		os.Exit(1)
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

	c, err := azure.NewStorageClient(storageAccount, storageAccessKey)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	blobName := azure.EnsureVHDExtension(path.Base(fileName))

	err = c.UploadPageBlob(
		azure.BlobMetadata{
			StorageAccount: storageAccount,
			BlobName:       blobName,
			ContainerName:  containerName,
		},
		fileName,
		threads,
	)

	if err != nil {
		fmt.Println("Error: ", err)
	}
}

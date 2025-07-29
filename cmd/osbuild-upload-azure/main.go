package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/osbuild/images/pkg/upload/azure"
)

func checkStringNotEmpty(variable string, errorMessage string) {
	if variable == "" {
		fmt.Fprintln(os.Stderr, errorMessage)
		flag.Usage()
		os.Exit(1)
	}
}

type tags map[string]string

func (t *tags) String() string {
	return ""
}

func (t *tags) Set(value string) error {
	splitValue := strings.SplitN(value, ":", 2)
	if len(splitValue) < 2 {
		return fmt.Errorf(`-tag must be in format key:value, "%s" is not valid`, value)
	}
	key := splitValue[0]
	val := splitValue[1]
	(*t)[key] = val

	return nil
}

func main() {
	var storageAccount string
	var storageAccessKey string
	var fileName string
	var containerName string
	var threads int
	tagsArg := tags(make(map[string]string))
	flag.StringVar(&storageAccount, "storage-account", "", "Azure storage account (mandatory)")
	flag.StringVar(&storageAccessKey, "storage-access-key", "", "Azure storage access key (mandatory)")
	flag.StringVar(&fileName, "image", "", "image to upload (mandatory)")
	flag.StringVar(&containerName, "container", "", "name of storage container (see Azure docs for explanation, mandatory)")
	flag.IntVar(&threads, "threads", 16, "number of threads for parallel upload")
	flag.Var(&tagsArg, "tag", "blob tag formatted as key:value (first colon found is considered to be the delimiter), can be specified multiple times")
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
	blobMetadata := azure.BlobMetadata{
		StorageAccount: storageAccount,
		BlobName:       blobName,
		ContainerName:  containerName,
	}
	err = c.UploadPageBlob(
		blobMetadata,
		fileName,
		threads,
	)
	if err != nil {
		fmt.Println("Uploading error: ", err)
		os.Exit(1)
	}

	err = c.TagBlob(context.Background(), blobMetadata, tagsArg)
	if err != nil {
		fmt.Println("Tagging error: ", err)
		os.Exit(1)
	}
}

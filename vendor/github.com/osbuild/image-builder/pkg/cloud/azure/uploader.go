package azure

import (
	"context"
	"fmt"
	"io"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/cloud"
)

const uploaderStorageContainer = "images"

var _ cloud.Uploader = &azureUploader{}

type azureUploader struct {
	client        *Client
	resourceGroup string
	imageName     string
	imagePath     string
	architecture  arch.Arch
}

func NewUploader(clientID, clientSecret, tenant, subscription, resourceGroup, imageName, imagePath string, architecture arch.Arch) (cloud.Uploader, error) {
	creds := Credentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	client, err := NewClient(creds, tenant, subscription)
	if err != nil {
		return nil, err
	}

	return &azureUploader{
		client:        client,
		resourceGroup: resourceGroup,
		imageName:     imageName,
		imagePath:     imagePath,
		architecture:  architecture,
	}, nil
}

func (au *azureUploader) Check(status io.Writer) error {
	ctx := context.Background()
	fmt.Fprintf(status, "Checking Azure resource group...\n")
	_, err := au.client.GetResourceGroupLocation(ctx, au.resourceGroup)
	if err != nil {
		return fmt.Errorf("cannot access resource group %q: %w", au.resourceGroup, err)
	}
	fmt.Fprintf(status, "Upload conditions met.\n")
	return nil
}

func (au *azureUploader) UploadAndRegister(_ io.Reader, _ uint64, status io.Writer) (*cloud.UploadResult, error) {
	ctx := context.Background()

	location, err := au.client.GetResourceGroupLocation(ctx, au.resourceGroup)
	if err != nil {
		return nil, err
	}

	staccTag := Tag{
		Name:  "imagesStorageAccount",
		Value: fmt.Sprintf("location=%s", location),
	}
	stacc, err := au.client.GetResourceNameByTag(ctx, au.resourceGroup, staccTag)
	if err != nil {
		return nil, err
	}

	if stacc == "" {
		stacc = RandomStorageAccountName("images")
		fmt.Fprintf(status, "Creating storage account %s...\n", stacc)
		err = au.client.CreateStorageAccount(ctx, au.resourceGroup, stacc, "", staccTag)
		if err != nil {
			return nil, err
		}
	}

	storekey, err := au.client.GetStorageAccountKey(ctx, au.resourceGroup, stacc)
	if err != nil {
		return nil, err
	}

	storeClient, err := NewStorageClient(stacc, storekey)
	if err != nil {
		return nil, err
	}

	err = storeClient.CreateStorageContainerIfNotExist(ctx, stacc, uploaderStorageContainer)
	if err != nil {
		return nil, err
	}

	blobName := EnsureVHDExtension(au.imageName)
	fmt.Fprintf(status, "Uploading %s to Azure...\n", blobName)
	err = storeClient.UploadPageBlob(
		BlobMetadata{
			StorageAccount: stacc,
			ContainerName:  uploaderStorageContainer,
			BlobName:       blobName,
		},
		au.imagePath,
		DefaultUploadThreads,
	)
	if err != nil {
		return nil, err
	}

	switch au.architecture {
	case arch.ARCH_X86_64:
		fmt.Fprintf(status, "Registering image %s...\n", au.imageName)
		err = au.client.RegisterImage(
			ctx,
			au.resourceGroup,
			stacc,
			uploaderStorageContainer,
			blobName,
			au.imageName,
			"",
			HyperVGenV2,
		)
		if err != nil {
			return nil, err
		}
		imageID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s",
			au.client.SubscriptionID(), au.resourceGroup, au.imageName)
		fmt.Fprintf(status, "Image registered: %s\n", imageID)
		return &cloud.UploadResult{
			Provider: "azure",
			ImageID:  imageID,
		}, nil
	case arch.ARCH_AARCH64:
		fmt.Fprintf(status, "Registering gallery image %s...\n", au.imageName)
		gi, err := au.client.RegisterGalleryImage(
			ctx,
			au.resourceGroup,
			stacc,
			uploaderStorageContainer,
			blobName,
			au.imageName,
			"",
			HyperVGenV2,
			arch.ARCH_AARCH64,
		)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(status, "Gallery image registered: %s\n", gi.ImageRef)
		return &cloud.UploadResult{
			Provider: "azure",
			ImageID:  gi.ImageRef,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture %q for Azure upload", au.architecture)
	}
}

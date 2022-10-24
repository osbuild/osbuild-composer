package azure

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

type Client struct {
	authorizer autorest.Authorizer
}

// NewClient creates a client for accessing the Azure API.
// See https://docs.microsoft.com/en-us/rest/api/azure/
// If you need to work with the Azure Storage API, see NewStorageClient
func NewClient(credentials Credentials, tenantID string) (*Client, error) {
	credentialsConfig := auth.NewClientCredentialsConfig(credentials.clientID, credentials.clientSecret, tenantID)
	authorizer, err := credentialsConfig.Authorizer()
	if err != nil {
		return nil, fmt.Errorf("creating an azure authorizer failed: %v", err)
	}

	return &Client{
		authorizer: authorizer,
	}, nil
}

// Tag is a name-value pair representing the tag structure in Azure
type Tag struct {
	Name  string
	Value string
}

// GetResourceNameByTag returns a name of an Azure resource tagged with the
// given `tag`. Note that if multiple resources with the same tag exists
// in the specified resource group, only one name is returned. It's undefined
// which one it is.
func (ac Client) GetResourceNameByTag(ctx context.Context, subscriptionID, resourceGroup string, tag Tag) (string, error) {
	c := resources.NewClient(subscriptionID)
	c.Authorizer = ac.authorizer

	filter := fmt.Sprintf("tagName eq '%s' and tagValue eq '%s'", tag.Name, tag.Value)
	result, err := c.ListByResourceGroup(ctx, resourceGroup, filter, "", nil)
	if err != nil {
		return "", fmt.Errorf("listing resources failed: %v", err)
	}

	if len(result.Values()) < 1 {
		return "", nil
	}

	return *result.Values()[0].Name, nil
}

// GetResourceGroupLocation returns the location of the given resource group.
func (ac Client) GetResourceGroupLocation(ctx context.Context, subscriptionID, resourceGroup string) (string, error) {
	c := resources.NewGroupsClient(subscriptionID)
	c.Authorizer = ac.authorizer

	group, err := c.Get(ctx, resourceGroup)
	if err != nil {
		return "", fmt.Errorf("retrieving resource group failed: %v", err)
	}

	return *group.Location, nil
}

// CreateStorageAccount creates a storage account in the specified resource
// group. The location parameter can be used to specify its location. The tag
// can be used to specify a tag attached to the account.
// The location is optional and if not provided, it is determined
// from the resource group.
func (ac Client) CreateStorageAccount(ctx context.Context, subscriptionID, resourceGroup, name, location string, tag Tag) error {
	c := storage.NewAccountsClient(subscriptionID)
	c.Authorizer = ac.authorizer

	var err error
	if location == "" {
		location, err = ac.GetResourceGroupLocation(ctx, subscriptionID, resourceGroup)
		if err != nil {
			return fmt.Errorf("retrieving resource group location failed: %v", err)
		}
	}

	result, err := c.Create(ctx, resourceGroup, name, storage.AccountCreateParameters{
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
			Tier: storage.Standard,
		},
		Location: &location,
		Tags: map[string]*string{
			tag.Name: &tag.Value,
		},
	})
	if err != nil {
		return fmt.Errorf("sending the create storage account request failed: %v", err)
	}

	err = result.WaitForCompletionRef(ctx, c.Client)
	if err != nil {
		return fmt.Errorf("waiting for the create storage account request failed: %v", err)
	}

	_, err = result.Result(c)
	if err != nil {
		return fmt.Errorf("create storage account request failed: %v", err)
	}

	return nil
}

// GetStorageAccountKey returns a storage account key that can be used to
// access the given storage account. This method always returns only the first
// key.
func (ac Client) GetStorageAccountKey(ctx context.Context, subscriptionID, resourceGroup string, storageAccount string) (string, error) {
	c := storage.NewAccountsClient(subscriptionID)
	c.Authorizer = ac.authorizer

	keys, err := c.ListKeys(ctx, resourceGroup, storageAccount)
	if err != nil {
		return "", fmt.Errorf("retrieving keys for a storage account failed: %v", err)
	}

	if len(*keys.Keys) == 0 {
		return "", errors.New("azure returned an empty list of keys")
	}

	return *(*keys.Keys)[0].Value, nil
}

// RegisterImage creates a generalized V1 Linux image from a given blob.
// The location is optional and if not provided, it is determined
// from the resource group.
func (ac Client) RegisterImage(ctx context.Context, subscriptionID, resourceGroup, storageAccount, storageContainer, blobName, imageName, location string) error {
	c := compute.NewImagesClient(subscriptionID)
	c.Authorizer = ac.authorizer

	blobURI := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", storageAccount, storageContainer, blobName)

	var err error
	if location == "" {
		location, err = ac.GetResourceGroupLocation(ctx, subscriptionID, resourceGroup)
		if err != nil {
			return fmt.Errorf("retrieving resource group location failed: %v", err)
		}
	}

	imageFuture, err := c.CreateOrUpdate(ctx, resourceGroup, imageName, compute.Image{
		Response: autorest.Response{},
		ImageProperties: &compute.ImageProperties{
			SourceVirtualMachine: nil,
			StorageProfile: &compute.ImageStorageProfile{
				OsDisk: &compute.ImageOSDisk{
					OsType:  compute.Linux,
					BlobURI: &blobURI,
					OsState: compute.Generalized,
				},
			},
		},
		Location: &location,
	})
	if err != nil {
		return fmt.Errorf("sending the create image request failed: %v", err)
	}

	err = imageFuture.WaitForCompletionRef(ctx, c.Client)
	if err != nil {
		return fmt.Errorf("waiting for the create image request failed: %v", err)
	}

	_, err = imageFuture.Result(c)
	if err != nil {
		return fmt.Errorf("create image request failed: %v", err)
	}

	return nil
}

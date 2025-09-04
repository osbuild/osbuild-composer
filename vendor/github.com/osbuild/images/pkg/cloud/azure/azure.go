package azure

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"

	"github.com/osbuild/images/internal/common"
)

type HyperVGenerationType string

const (
	HyperVGenV1 HyperVGenerationType = "V1"
	HyperVGenV2 HyperVGenerationType = "V2"
)

type Client struct {
	creds          *azidentity.ClientSecretCredential
	resources      ResourcesClient
	resourceGroups ResourceGroupsClient
	accounts       AccountsClient
	images         ImagesClient
	vms            VMsClient
	disks          DisksClient
	vnets          VirtualNetworksClient
	subnets        SubnetsClient
	securityGroups SecurityGroupsClient
	publicIPs      PublicIPsClient
	interfaces     InterfacesClient
}

func newTestClient(
	rc ResourcesClient,
	rgc ResourceGroupsClient,
	ac AccountsClient,
	ic ImagesClient,
	vnets VirtualNetworksClient,
	subnets SubnetsClient,
	pips PublicIPsClient,
	sgs SecurityGroupsClient,
	intfs InterfacesClient,
	vms VMsClient,
	disks DisksClient,
) *Client {
	return &Client{
		creds:          nil,
		resources:      rc,
		resourceGroups: rgc,
		accounts:       ac,
		images:         ic,
		vnets:          vnets,
		subnets:        subnets,
		publicIPs:      pips,
		securityGroups: sgs,
		interfaces:     intfs,
		vms:            vms,
		disks:          disks,
	}
}

// NewClient creates a client for accessing the Azure API.
// See https://docs.microsoft.com/en-us/rest/api/azure/
// If you need to work with the Azure Storage API, see NewStorageClient
func NewClient(credentials Credentials, tenantID, subscriptionID string) (*Client, error) {
	creds, err := azidentity.NewClientSecretCredential(tenantID, credentials.ClientID, credentials.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("creating azure ClientSecretCredential failed: %w", err)
	}

	resFact, err := armresources.NewClientFactory(subscriptionID, creds, nil)
	if err != nil {
		return nil, fmt.Errorf("creating resources client factory failed: %w", err)
	}

	storFact, err := armstorage.NewClientFactory(subscriptionID, creds, nil)
	if err != nil {
		return nil, fmt.Errorf("creating storage client factory failed: %w", err)
	}

	compFact, err := armcompute.NewClientFactory(subscriptionID, creds, nil)
	if err != nil {
		return nil, fmt.Errorf("creating compute client factory failed: %w", err)
	}

	networkFact, err := armnetwork.NewClientFactory(subscriptionID, creds, nil)
	if err != nil {
		return nil, fmt.Errorf("creating compute client factory failed: %w", err)
	}
	return &Client{
		creds,
		resFact.NewClient(),
		resFact.NewResourceGroupsClient(),
		storFact.NewAccountsClient(),
		compFact.NewImagesClient(),
		compFact.NewVirtualMachinesClient(),
		compFact.NewDisksClient(),
		networkFact.NewVirtualNetworksClient(),
		networkFact.NewSubnetsClient(),
		networkFact.NewSecurityGroupsClient(),
		networkFact.NewPublicIPAddressesClient(),
		networkFact.NewInterfacesClient(),
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
func (ac Client) GetResourceNameByTag(ctx context.Context, resourceGroup string, tag Tag) (string, error) {
	pager := ac.resources.NewListByResourceGroupPager(resourceGroup, &armresources.ClientListByResourceGroupOptions{
		Filter: common.ToPtr(fmt.Sprintf("tagName eq '%s' and tagValue eq '%s'", tag.Name, tag.Value)),
	})

	result, err := pager.NextPage(ctx)
	if err != nil {
		return "", fmt.Errorf("listing resources failed: %w", err)
	}

	if len(result.Value) < 1 {
		return "", nil
	}
	return *result.Value[0].Name, nil
}

// GetResourceGroupLocation returns the location of the given resource group.
func (ac Client) GetResourceGroupLocation(ctx context.Context, resourceGroup string) (string, error) {
	group, err := ac.resourceGroups.Get(ctx, resourceGroup, nil)
	if err != nil {
		return "", fmt.Errorf("retrieving resource group failed: %w", err)
	}

	return *group.Location, nil
}

// CreateStorageAccount creates a storage account in the specified resource
// group. The location parameter can be used to specify its location. The tag
// can be used to specify a tag attached to the account.
// The location is optional and if not provided, it is determined
// from the resource group.
func (ac Client) CreateStorageAccount(ctx context.Context, resourceGroup, name, location string, tag Tag) error {
	var err error
	if location == "" {
		location, err = ac.GetResourceGroupLocation(ctx, resourceGroup)
		if err != nil {
			return fmt.Errorf("retrieving resource group location failed: %w", err)
		}
	}

	poller, err := ac.accounts.BeginCreate(ctx, resourceGroup, name, armstorage.AccountCreateParameters{
		SKU: &armstorage.SKU{
			Name: common.ToPtr(armstorage.SKUNameStandardLRS),
			Tier: common.ToPtr(armstorage.SKUTierStandard),
		},
		Location: &location,
		Tags: map[string]*string{
			tag.Name: &tag.Value,
		},
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess: common.ToPtr(false),
			MinimumTLSVersion:     common.ToPtr(armstorage.MinimumTLSVersionTLS12),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("sending the create storage account request failed: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create storage account request failed: %w", err)
	}

	return nil
}

// GetStorageAccountKey returns a storage account key that can be used to
// access the given storage account. This method always returns only the first
// key.
func (ac Client) GetStorageAccountKey(ctx context.Context, resourceGroup string, storageAccount string) (string, error) {
	keys, err := ac.accounts.ListKeys(ctx, resourceGroup, storageAccount, nil)
	if err != nil {
		return "", fmt.Errorf("retrieving keys for a storage account failed: %w", err)
	}

	if len(keys.Keys) == 0 {
		return "", errors.New("azure returned an empty list of keys")
	}

	return *keys.Keys[0].Value, nil
}

// RegisterImage creates a generalized V1 Linux image from a given blob.
// The location is optional and if not provided, it is determined
// from the resource group.
func (ac Client) RegisterImage(ctx context.Context, resourceGroup, storageAccount, storageContainer, blobName, imageName, location string, hyperVGen HyperVGenerationType) error {
	blobURI := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", storageAccount, storageContainer, blobName)

	var err error
	if location == "" {
		location, err = ac.GetResourceGroupLocation(ctx, resourceGroup)
		if err != nil {
			return fmt.Errorf("retrieving resource group location failed: %w", err)
		}
	}

	var hypvgen armcompute.HyperVGenerationTypes
	switch hyperVGen {
	case HyperVGenV1:
		hypvgen = armcompute.HyperVGenerationTypes(armcompute.HyperVGenerationTypesV1)
	case HyperVGenV2:
		hypvgen = armcompute.HyperVGenerationTypes(armcompute.HyperVGenerationTypesV2)
	default:
		return fmt.Errorf("Unknown hyper v generation type %v", hyperVGen)
	}

	imageFuture, err := ac.images.BeginCreateOrUpdate(ctx, resourceGroup, imageName, armcompute.Image{
		Properties: &armcompute.ImageProperties{
			HyperVGeneration:     common.ToPtr(hypvgen),
			SourceVirtualMachine: nil,
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:  common.ToPtr(armcompute.OperatingSystemTypesLinux),
					BlobURI: &blobURI,
					OSState: common.ToPtr(armcompute.OperatingSystemStateTypesGeneralized),
				},
			},
		},
		Location: &location,
	}, nil)
	if err != nil {
		return fmt.Errorf("sending the create image request failed: %w", err)
	}

	_, err = imageFuture.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create image request failed: %w", err)
	}

	return nil
}

func (ac Client) DeleteImage(ctx context.Context, resourceGroup, imageName string) error {
	poller, err := ac.images.BeginDelete(ctx, resourceGroup, imageName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

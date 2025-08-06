package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

type ResourcesClient interface {
	NewListByResourceGroupPager(string, *armresources.ClientListByResourceGroupOptions) *runtime.Pager[armresources.ClientListByResourceGroupResponse]
}

type ResourceGroupsClient interface {
	Get(context.Context, string, *armresources.ResourceGroupsClientGetOptions) (armresources.ResourceGroupsClientGetResponse, error)
}

type AccountsClient interface {
	BeginCreate(context.Context, string, string, armstorage.AccountCreateParameters, *armstorage.AccountsClientBeginCreateOptions) (*runtime.Poller[armstorage.AccountsClientCreateResponse], error)
	ListKeys(context.Context, string, string, *armstorage.AccountsClientListKeysOptions) (armstorage.AccountsClientListKeysResponse, error)
}

type ImagesClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armcompute.Image, *armcompute.ImagesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.ImagesClientCreateOrUpdateResponse], error)
}

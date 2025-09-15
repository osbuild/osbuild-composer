package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
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
	BeginDelete(context.Context, string, string, *armcompute.ImagesClientBeginDeleteOptions) (*runtime.Poller[armcompute.ImagesClientDeleteResponse], error)
}

type VMsClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armcompute.VirtualMachine, *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armcompute.VirtualMachinesClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeleteResponse], error)
}

type DisksClient interface {
	BeginDelete(context.Context, string, string, *armcompute.DisksClientBeginDeleteOptions) (*runtime.Poller[armcompute.DisksClientDeleteResponse], error)
}

type GalleriesClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armcompute.Gallery, *armcompute.GalleriesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleriesClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armcompute.GalleriesClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleriesClientDeleteResponse], error)
}

type GalleryImagesClient interface {
	BeginCreateOrUpdate(context.Context, string, string, string, armcompute.GalleryImage, *armcompute.GalleryImagesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleryImagesClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, string, *armcompute.GalleryImagesClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleryImagesClientDeleteResponse], error)
}

type GalleryImageVersionsClient interface {
	BeginCreateOrUpdate(context.Context, string, string, string, string, armcompute.GalleryImageVersion, *armcompute.GalleryImageVersionsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleryImageVersionsClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, string, string, *armcompute.GalleryImageVersionsClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleryImageVersionsClientDeleteResponse], error)
}

type VirtualNetworksClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armnetwork.VirtualNetwork, *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armnetwork.VirtualNetworksClientBeginDeleteOptions) (*runtime.Poller[armnetwork.VirtualNetworksClientDeleteResponse], error)
}

type SubnetsClient interface {
	BeginCreateOrUpdate(context.Context, string, string, string, armnetwork.Subnet, *armnetwork.SubnetsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SubnetsClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, string, *armnetwork.SubnetsClientBeginDeleteOptions) (*runtime.Poller[armnetwork.SubnetsClientDeleteResponse], error)
}

type SecurityGroupsClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armnetwork.SecurityGroup, *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armnetwork.SecurityGroupsClientBeginDeleteOptions) (*runtime.Poller[armnetwork.SecurityGroupsClientDeleteResponse], error)
}

type PublicIPsClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armnetwork.PublicIPAddress, *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armnetwork.PublicIPAddressesClientBeginDeleteOptions) (*runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse], error)
}

type InterfacesClient interface {
	BeginCreateOrUpdate(context.Context, string, string, armnetwork.Interface, *armnetwork.InterfacesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.InterfacesClientCreateOrUpdateResponse], error)
	BeginDelete(context.Context, string, string, *armnetwork.InterfacesClientBeginDeleteOptions) (*runtime.Poller[armnetwork.InterfacesClientDeleteResponse], error)
}
